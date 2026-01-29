package session

type StreamDelta struct {
	Text string `json:"text"`
}

type StreamDone struct {
	Text string `json:"text"`
}

type StreamError struct {
	Message string `json:"message"`
}
