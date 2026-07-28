package main

import (
	"bytes"
	"encoding/binary"
	"math"
)

type ChimeType int

const (
	ChimeToBreak ChimeType = iota // Focus -> Break (E5, A5, C#6)
	ChimeToFocus                  // Break -> Focus (A4, E5)
)

var (
	wavChimeToBreak []byte
	wavChimeToFocus []byte
)

func init() {
	wavChimeToBreak = generateChimeWav([]noteSpec{
		{freq: 659.25, start: 0.00, decay: 3.0, volume: 0.80},  // E5
		{freq: 880.00, start: 0.15, decay: 2.8, volume: 0.85},  // A5
		{freq: 1108.73, start: 0.30, decay: 2.5, volume: 0.90}, // C#6
	}, 1.6)

	wavChimeToFocus = generateChimeWav([]noteSpec{
		{freq: 440.00, start: 0.00, decay: 3.5, volume: 0.80}, // A4
		{freq: 659.25, start: 0.18, decay: 3.0, volume: 0.85}, // E5
	}, 1.3)
}

type noteSpec struct {
	freq   float64
	start  float64
	decay  float64
	volume float64
}

func PlayChime(chime ChimeType, volume int) {
	if volume <= 0 {
		return
	}
	gain := float64(volume) / 100.0
	switch chime {
	case ChimeToBreak:
		playAudioBytes(applyVolume(wavChimeToBreak, gain))
	case ChimeToFocus:
		playAudioBytes(applyVolume(wavChimeToFocus, gain))
	}
}

func applyVolume(wav []byte, gain float64) []byte {
	if len(wav) <= 44 || gain >= 1.0 {
		return wav
	}
	out := make([]byte, len(wav))
	copy(out[:44], wav[:44])

	for i := 44; i+1 < len(wav); i += 2 {
		val := int16(binary.LittleEndian.Uint16(wav[i : i+2]))
		scaled := int16(float64(val) * gain)
		binary.LittleEndian.PutUint16(out[i:], uint16(scaled))
	}
	return out
}

func generateChimeWav(notes []noteSpec, duration float64) []byte {
	sampleRate := 44100
	numSamples := int(float64(sampleRate) * duration)
	samples := make([]float64, numSamples)

	for _, note := range notes {
		startIndex := int(note.start * float64(sampleRate))
		for i := startIndex; i < numSamples; i++ {
			t := float64(i-startIndex) / float64(sampleRate)
			envelope := math.Exp(-note.decay * t)
			tone := math.Sin(2*math.Pi*note.freq*t) +
				0.25*math.Sin(2*math.Pi*note.freq*2*t) +
				0.08*math.Sin(2*math.Pi*note.freq*3*t)

			samples[i] += tone * envelope * note.volume
		}
	}

	pcmData := make([]byte, numSamples*2)
	for i, sample := range samples {
		if sample > 1.0 {
			sample = 1.0
		} else if sample < -1.0 {
			sample = -1.0
		}
		val := int16(sample * 32767.0)
		binary.LittleEndian.PutUint16(pcmData[i*2:], uint16(val))
	}

	var buf bytes.Buffer
	buf.WriteString("RIFF")
	binary.Write(&buf, binary.LittleEndian, uint32(36+len(pcmData)))
	buf.WriteString("WAVEfmt ")
	binary.Write(&buf, binary.LittleEndian, uint32(16))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(&buf, binary.LittleEndian, uint32(sampleRate*2))
	binary.Write(&buf, binary.LittleEndian, uint16(2))
	binary.Write(&buf, binary.LittleEndian, uint16(16))
	buf.WriteString("data")
	binary.Write(&buf, binary.LittleEndian, uint32(len(pcmData)))
	buf.Write(pcmData)

	return buf.Bytes()
}
