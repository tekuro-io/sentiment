package sentiment

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/polygon-io/client-go/rest/models"
)

type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	return &SSEWriter{w, flusher}, nil
}

func (s *SSEWriter) WriteEvent(data string) error {
	_, err := fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flusher.Flush()
	return err
}

func (s *SSEWriter) WriteNews(newsList []models.TickerNews) error {
	var err error
	for _, news := range newsList {
		jsonBytes, err := json.Marshal(news)
		if err != nil {
			s.Error(err)
		}
		_, err = fmt.Fprintf(s.w, "data: %s\n\n", string(jsonBytes))
		s.flusher.Flush()
		if err != nil {
			log.Printf("error writing news to sse: %v\n", err)
			s.Error(err)
		}
	}
	return err
}

func (s *SSEWriter) WriteRanAt(ranAt time.Time) error {
	_, err := fmt.Fprintf(s.w, "data: %d\n\n", ranAt.UnixMilli())
	s.flusher.Flush()
	return err
}

func (s *SSEWriter) Overview() {
	fmt.Fprint(s.w, "data: [OVERVIEW]\n\n")
	s.flusher.Flush()
}

func (s *SSEWriter) PNews() {
	fmt.Fprint(s.w, "data: [PNEWS]\n\n")
	s.flusher.Flush()
}

func (s *SSEWriter) GNews() {
	fmt.Fprint(s.w, "data: [GNEWS]\n\n")
	s.flusher.Flush()
}

func (s *SSEWriter) Model() {
	fmt.Fprint(s.w, "data: [MODEL]\n\n")
	s.flusher.Flush()
}

func (s *SSEWriter) ModelBegin() {
	fmt.Fprint(s.w, "data: [MODELBEGIN]\n\n")
	s.flusher.Flush()
}

func (s *SSEWriter) TickNews() {
	fmt.Fprint(s.w, "data: [TICKNEWS]\n\n")
	s.flusher.Flush()
}

func (s *SSEWriter) RanAt() {
	fmt.Fprint(s.w, "data: [RANAT]\n\n")
	s.flusher.Flush()
}

func (s *SSEWriter) Error(err error) {
	fmt.Fprintf(s.w, "data: [ERROR] %s\n\n", err.Error())
	s.flusher.Flush()
}

func (s *SSEWriter) Done() {
	fmt.Fprint(s.w, "data: [DONE]\n\n")
	s.flusher.Flush()
}
