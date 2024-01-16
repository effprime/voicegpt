package main

import (
	"io"
	"log"
	"net/http"

	"github.com/effprime/voicegpt/pkg/voicegpt"
	"github.com/gorilla/mux"
)

func main() {
	handler, err := voicegpt.NewVoiceGPTHandler(&voicegpt.VoiceGPTOptions{
		GPTModel: "gpt-4",
	})
	if err != nil {
		log.Fatalf("Error initializing VoiceGPTHandler: %v", err)
	}

	r := mux.NewRouter()

	r.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		log.Println("parsing multipart upload")

		err := r.ParseMultipartForm(10 << 20) // Max upload size set to 10 MB
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Println("fetching form file")
		file, _, err := r.FormFile("wavFile")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		log.Println("calling internal handler")
		response, err := handler.Handle(r.Context(), &voicegpt.Request{
			VoiceData: file,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Println("writing response")
		w.Header().Set("Content-Type", "audio/mpeg")
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
