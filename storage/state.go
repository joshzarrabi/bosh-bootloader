package storage

type State struct {
	Version        int     `json:"version"`
	IAAS           string  `json:"iaas"`
	ID             string  `json:"id"`
	NoDirector     bool    `json:"noDirector"`
	AWS            AWS     `json:"aws,omitempty"`
	Azure          Azure   `json:"azure,omitempty"`
	GCP            GCP     `json:"gcp,omitempty"`
	Jumpbox        Jumpbox `json:"jumpbox,omitempty"`
	BOSH           BOSH    `json:"bosh,omitempty"`
	EnvID          string  `json:"envID"`
	TFState        string  `json:"tfState"`
	LB             LB      `json:"lb"`
	LatestTFOutput string  `json:"latestTFOutput"`
}
