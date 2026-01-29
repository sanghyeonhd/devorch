package bus

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func WriteSSE(w http.ResponseWriter, data any) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", string(b))
	if err != nil {
		return err
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func WriteSSEComment(w http.ResponseWriter, s string) {
	fmt.Fprintf(w, ": %s\n\n", s)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
