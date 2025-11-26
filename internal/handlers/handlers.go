package handlers

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"github.com/a-h/templ"
	"github.com/starfederation/datastar-go/datastar"
)

const (
	SessionCookie = "session"
)

type Handlers struct {
	mu          sync.Mutex
	logger      *slog.Logger
	connections map[string]chan any
}

func New(logger *slog.Logger) *Handlers {
	return &Handlers{
		logger:      logger,
		connections: map[string]chan any{},
	}
}

func (h *Handlers) internalServerError(w http.ResponseWriter, err error) {
	h.logger.Error("internal server error", "error", err.Error())
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

type ViewRequest struct {
	Content string
	URL     url.URL
	Signals any
}

type SignalsUpdate struct {
	Signals any
}

func (h *Handlers) APIEndpoint() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookie)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusFound)
		}

		sse := datastar.NewSSE(w, r, datastar.WithCompression(datastar.WithBrotli()))
		sessionID := cookie.Value
		ch := make(chan any)

		h.mu.Lock()
		h.connections[sessionID] = ch
		h.mu.Unlock()

		defer func() {
			close(ch)
			h.mu.Lock()
			delete(h.connections, sessionID)
			h.mu.Unlock()
		}()

		for {
			select {
			case <-r.Context().Done():
				return
			case event := <-ch:
				switch e := event.(type) {
				case SignalsUpdate:
					if e.Signals != nil {
						if err = sse.MarshalAndPatchSignals(e.Signals); err != nil {
							h.logger.Error("failed to marshal signals", "error", err.Error())
						}
					}
				case ViewRequest:
					if err = sse.PatchElements(e.Content); err != nil {
						h.logger.Error("failed to patch elements", "error", err.Error())
					}
					if err = sse.ReplaceURL(e.URL); err != nil {
						h.logger.Error("failed to replace URL", "error", err.Error())
					}
					if e.Signals != nil {
						if err = sse.MarshalAndPatchSignals(e.Signals); err != nil {
							h.logger.Error("failed to marshal signals", "error", err.Error())
						}
					}
				}
			}
		}
	}
}

func (h *Handlers) renderView(component templ.Component, signals any, w http.ResponseWriter, r *http.Request) {
	session, err := r.Cookie(SessionCookie)
	if err != nil {
		h.internalServerError(w, err)
		return
	}

	ch := h.connections[session.Value]
	buf := &bytes.Buffer{}
	if err = component.Render(r.Context(), buf); err != nil {
		h.internalServerError(w, err)
	}

	q := r.URL.Query()
	q.Del("datastar")
	r.URL.RawQuery = q.Encode()
	h.logger.Info(r.URL.String())

	event := ViewRequest{
		Content: buf.String(),
		URL:     *r.URL,
		Signals: signals,
	}

	go func() {
		ch <- event
	}()

	w.WriteHeader(http.StatusNoContent)
}
