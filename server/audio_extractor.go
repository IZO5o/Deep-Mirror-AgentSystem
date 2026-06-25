package server

import (
	"context"
	"fmt"
	"os/exec"
)

type AudioExtractor interface {
	ExtractAudio(ctx context.Context, inputPath string, outputPath string) error
}

type FFmpegAudioExtractor struct {
	path string
}

func NewFFmpegAudioExtractor(path string) *FFmpegAudioExtractor {
	if path == "" {
		path = "ffmpeg"
	}
	return &FFmpegAudioExtractor{path: path}
}

func (e *FFmpegAudioExtractor) ExtractAudio(ctx context.Context, inputPath string, outputPath string) error {
	cmd := exec.CommandContext(ctx, e.path, "-y", "-i", inputPath, "-vn", "-ac", "1", "-ar", "16000", outputPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg extract audio failed: %w: %s", err, string(output))
	}
	return nil
}
