package benchmark

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
)

// ContainerPool is a thread-safe container pool structure.
type ContainerPool struct {
	containers chan testcontainers.Container
}

type CreateContainer func() (testcontainers.Container, error)

// StartContainerPool initializes a pool of containers.
func StartContainerPool(poolSize int, producer CreateContainer) (*ContainerPool, error) {
	pool := &ContainerPool{
		containers: make(chan testcontainers.Container, poolSize),
	}

	var wg sync.WaitGroup
	for i := 0; i < cap(pool.containers); i++ {
		wg.Add(1)
		go func(ch chan testcontainers.Container) {
			defer wg.Done()
			container, err := producer()
			if err != nil {
				panic(fmt.Errorf("failed to create container: %w", err))
			}

			// Add the container to the pool
			ch <- container
		}(pool.containers)
	}

	wg.Wait()

	return pool, nil
}

// BorrowContainer retrieves a container from the pool.
func (p *ContainerPool) BorrowContainer(ctx context.Context) (testcontainers.Container, error) {
	logrus.Debug("wait for container")
	select {
	case container := <-p.containers:
		logrus.Debug("borrowed container")
		return container, nil
	case <-ctx.Done():
		return nil, ctx.Err() // Return error if context is cancelled
	}
}

// ReturnContainer puts the container back into the pool.
func (p *ContainerPool) ReturnContainer(container testcontainers.Container) {
	logrus.Debug("return container")
	p.containers <- container
}

// TerminatePool terminates all containers in the pool.
func (p *ContainerPool) TerminatePool(ctx context.Context) {

	var wg sync.WaitGroup
	for i := 0; i < cap(p.containers); i++ {
		wg.Add(1)
		go func(ch chan testcontainers.Container) {
			defer wg.Done()
			container := <-ch
			_ = container.Terminate(ctx)
		}(p.containers)
	}

	wg.Wait()
}

// ExecuteCommandInContainer borrows a container, runs a command, and returns it to the pool.
func (p *ContainerPool) ExecuteCommandInContainer(ctx context.Context, command ...string) error {
	container, err := p.BorrowContainer(ctx)
	if err != nil {
		return fmt.Errorf("failed to borrow container: %w", err)
	}

	defer p.ReturnContainer(container)

	// Execute the provided command inside the borrowed container
	exitCode, reader, err := container.Exec(ctx, command)
	readerConsumer := discard

	if logrus.IsLevelEnabled(logrus.DebugLevel) || exitCode != 0 {
		readerConsumer = toLogrus
	}

	// consume reader to prevent OOM
	go readerConsumer(reader)

	if err != nil || exitCode != 0 {
		return fmt.Errorf("failed to execute command: %w, exit code: %d", err, exitCode)
	}
	return nil
}

func toLogrus(reader io.Reader) {
	if reader == nil {
		return
	}
	if b, err := io.ReadAll(reader); err == nil {
		logrus.Println(string(b))
	}
}

func discard(reader io.Reader) {
	if reader == nil {
		return
	}
	_, _ = io.Copy(io.Discard, reader)
}
