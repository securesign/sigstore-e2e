package testsupport

type RekorCLIVerifyOutput struct {
	RekorHash  string
	EntryIndex int
}

type RekorCLIGetOutput struct {
	HashedRekordObj struct {
		Data struct {
			Hash struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"hash"`
		} `json:"data"`
		Signature struct {
			Content   string `json:"content"`
			PublicKey struct {
				Content string `json:"content"`
			} `json:"publicKey"`
		} `json:"signature"`
	} `json:"HashedRekordObj"`
	RekordObj struct {
		Data struct {
			Hash struct {
				Algorithm string `json:"algorithm"`
				Value     string `json:"value"`
			} `json:"hash"`
		} `json:"data"`
		Signature struct {
			Content   string `json:"content"`
			PublicKey struct {
				Content string `json:"content"`
			} `json:"publicKey"`
		} `json:"signature"`
	} `json:"RekordObj"`
}

type CosignVerifyOutput []struct {
	Optional struct {
		Bundle struct {
			Payload struct {
				LogIndex int `json:"logIndex"`
			} `json:"Payload"`
		} `json:"Bundle"`
	} `json:"optional"`
}
