package queryapi

type QueryResponse struct {
	Results []Result `json:"results"`
}

type Result struct {
	Columns []string `json:"columns"`
	Data    []Row    `json:"data"`
}

type Row struct {
	Row []json.RawMessage `json:"row"`
}
