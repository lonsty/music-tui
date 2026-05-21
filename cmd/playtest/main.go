// playtest is a minimal CLI to verify beep can open and play an MP3.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: playtest <file.mp3>")
		os.Exit(1)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}

	streamer, format, err := mp3.Decode(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, "decode:", err)
		os.Exit(1)
	}
	defer streamer.Close()

	fmt.Printf("sample rate : %d Hz\n", format.SampleRate)
	fmt.Printf("channels    : %d\n", format.NumChannels)
	fmt.Printf("duration    : %s\n", format.SampleRate.D(streamer.Len()).Round(time.Second))

	if err := speaker.Init(format.SampleRate, format.SampleRate.N(100*time.Millisecond)); err != nil {
		fmt.Fprintln(os.Stderr, "speaker init:", err)
		os.Exit(1)
	}

	done := make(chan struct{})
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		close(done)
	})))

	fmt.Println("playing... (ctrl-c to stop)")
	<-done
	fmt.Println("done")
}
