package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/effprime/voicegpt/pkg/voicegpt"
	"github.com/gorilla/mux"
)

func main() {
	sessions, err := voicegpt.NewFileSessionStorage("/home/babybear/.voicegpt-sessions")
	if err != nil {
		log.Fatalf("Error getting file session storage: %v", err)
	}

	handler, err := voicegpt.NewVoiceGPTHandler(sessions, &voicegpt.VoiceGPTOptions{
		GPTModel: "gpt-4",
	})
	if err != nil {
		log.Fatalf("Error initializing VoiceGPTHandler: %v", err)
	}

	r := mux.NewRouter()

	r.HandleFunc("/session/{sessionId}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "X-Session-Id")
		vars := mux.Vars(r)
		sessionId := vars["sessionId"]

		session, err := sessions.Get(r.Context(), sessionId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(session)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}).Methods("GET")

	r.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Expose-Headers", "X-Session-Id")

		log.Println("Parsing multipart upload")

		err := r.ParseMultipartForm(10 << 20) // Max upload size set to 10 MB
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Println("Fetching form file")
		file, _, err := r.FormFile("file")
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		response, err := handler.Handle(r.Context(), &voicegpt.Request{
			SessionID: r.URL.Query().Get("session"),
			VoiceData: file,
		})
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Println("Writing response")
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("X-Session-Id", response.SessionID)
		if _, err := io.Copy(w, response.VoiceData); err != nil {
			log.Printf("Error writing MP3 data to response: %v", err)
			http.Error(w, "Error writing MP3 data", http.StatusInternalServerError)
			return
		}
	}).Methods("POST")

	// Start the server
	log.Println("Server is starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", r))
}
