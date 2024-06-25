package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
	flag "github.com/spf13/pflag"
)

type tts struct {
	tts *sherpa.OfflineTts
}

func New() *tts {
	config := &sherpa.OfflineTtsConfig{
		Model: sherpa.OfflineTtsModelConfig{
			Vits: sherpa.OfflineTtsVitsModelConfig{
				Model:       "./vits-coqui-en-vctk/model.onnx",
				Tokens:      "./vits-coqui-en-vctk/tokens.txt",
				DataDir:     "./vits-coqui-en-vctk/espeak-ng-data/",
				NoiseScale:  0.667,
				NoiseScaleW: 0.8,
				LengthScale: 1.0,
			},
			NumThreads: 1,
			Provider:   "cpu", // precompiled library doesn't have "cuda" :-(
		},
		MaxNumSentences: 1,
	}

	return &tts{
		tts: sherpa.NewOfflineTts(config),
	}
}

func (s *tts) Close() {
	sherpa.DeleteOfflineTts(s.tts)
}

func (s *tts) Generate(text string, sid int, speed float32) *sherpa.GeneratedAudio {
	return s.tts.Generate(text, sid, speed)
}

type Request struct {
	Text    string  `json:"text"`
	Speaker int     `json:"speaker"`
	Speed   float32 `json:"speed"`
}

func (s *tts) HandleGenerate(c *fiber.Ctx) error {
	var request Request

	if err := c.BodyParser(&request); err != nil {
		log.Printf("decoding request: %v", err)

		return c.SendStatus(fiber.StatusBadRequest)
	}

	if request.Text == "" {
		return fmt.Errorf("empty text")
	}

	if request.Speaker < 0 || request.Speaker > 108 {
		return fmt.Errorf("invalid speaker ID: %v", request.Speaker)
	}

	if request.Speed < 0.0 || request.Speed > 1.0 {
		return fmt.Errorf("invalid speed: %v", request.Speed)
	} else if request.Speed == 0.0 {
		request.Speed = 1.0
	}

	log.Printf("generating audio using speaker %d for %q",
		request.Speaker, request.Text)

	file, err := os.CreateTemp("", "tts*.wav")
	if err != nil {
		return fmt.Errorf("create tempfile: %v", err)
	}
	defer os.Remove(file.Name())

	audio := s.Generate(request.Text, request.Speaker, request.Speed)

	if !audio.Save(file.Name()) {
		return fmt.Errorf("failed to write %s", file.Name())
	}

	return c.SendFile(file.Name())
}

func (s *tts) HandleHealth(c *fiber.Ctx) error {
	return c.SendString("OK")
}

func (s *tts) ErrorHandler(c *fiber.Ctx, err error) error {
	c.Status(fiber.StatusInternalServerError)

	log.Printf("error: %v", err)

	return c.SendString(err.Error())
}

func cli(tts *tts, text string, sid int, filename string) {
	log.Println("Input text:", text)
	log.Println("Speaker ID:", sid)
	log.Println("Output filename:", filename)
	log.Println("Initializing model (may take several seconds)")
	log.Println("Model created!")
	log.Println("Start generating!")

	audio := tts.Generate(text, sid, 1.0)

	log.Println("Done!")

	ok := audio.Save(filename)
	if !ok {
		log.Fatal("Failed to write", filename)
	}
}

func server(tts *tts, addr string) {
	app := fiber.New(fiber.Config{
		EnablePrintRoutes: true,
		ErrorHandler:      tts.ErrorHandler,
	})

	app.Post("/", tts.HandleGenerate).Name("Generate")
	app.Get("/", tts.HandleHealth).Name("Health")

	if err := app.Listen(addr); err != nil {
		log.Fatalf("Failed to listen on %s: %v", addr, err)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	tts := New()
	defer tts.Close()

	sid := 0
	filename := "./generated.wav"
	addr := ""

	flag.IntVar(&sid, "sid", 0, "Speaker ID. Used only for multi-speaker models")
	flag.StringVar(&filename, "output-filename", "./generated.wav", "Filename to save the generated audio")
	flag.StringVar(&addr, "addr", "", "Address to listen for HTTP requests")

	flag.Parse()

	if addr == "" {
		if len(flag.Args()) != 1 {
			log.Fatalf("Please provide the text to generate audios")
		}

		cli(tts, flag.Arg(0), sid, filename)
	} else {
		server(tts, addr)
	}
}
