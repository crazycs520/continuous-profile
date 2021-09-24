package meta

type ProfileTarget struct {
	Tp      string `json:"type"`
	Job     string `json:"job"`
	Address string `json:"address"`
}

type BasicQueryParam struct {
	Begin   int64           `json:"begin_time"`
	End     int64           `json:"end_time"`
	Targets []ProfileTarget `json:"targets"`
}

type ProfileList struct {
	Target ProfileTarget `json:"target"`
	TsList []int64       `json:"timestamp_list"`
}
