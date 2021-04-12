package mixer

import (
	"crypto/tls"
	"github.com/spf13/cobra"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

var htmlPath string

var (
	ClientCmd = &cobra.Command{
		Use:   "cli",
		Short: "cli",
		RunE: func(cmd *cobra.Command, args []string) error {
			mux := http.NewServeMux()

			cfg := &tls.Config{
				MinVersion:               tls.VersionTLS12,
				CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
				PreferServerCipherSuites: true,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
					tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				},
			}
			server := &http.Server{
				Addr:         ":8000",
				Handler:      mux,
				TLSConfig:    cfg,
				TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler), 0),
			}

			// Serve browser for input
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				currentDir, err := os.Getwd()
				if err != nil {
					log.Fatalf("Error getting current directory: %s", err)
				}
				path := filepath.Join(currentDir, htmlPath)
				w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
				handler := http.FileServer(http.Dir(path))
				handler.ServeHTTP(w, r)
			})
			err := server.ListenAndServeTLS(os.Getenv("TLS_CERT"), os.Getenv("TLS_KEY"))
			if err != nil {
				log.Fatalf("ListenAndServeTLS: %s", err)
			}
			return nil
		},
	}
)