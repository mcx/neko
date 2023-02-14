package types

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/PaesslerAG/gval"
	"github.com/demodesk/neko/pkg/types/codec"
	"github.com/pion/webrtc/v3/pkg/media"
)

var (
	ErrCapturePipelineAlreadyExists = errors.New("capture pipeline already exists")
)

type Sample media.Sample

type Receiver interface {
	SetStream(stream StreamSinkManager) (changed bool, err error)
	RemoveStream()
	OnBitrateChange(f func(bitrate int) (changed bool, err error))
	OnVideoChange(f func(videoID string) (changed bool, err error))
	VideoAuto() bool
	SetVideoAuto(videoAuto bool)
}

type BucketsManager interface {
	IDs() []string
	Codec() codec.RTPCodec
	SetReceiver(receiver Receiver)
	RemoveReceiver(receiver Receiver) error

	DestroyAll()
	RecreateAll() error
	Shutdown()
}

type BroadcastManager interface {
	Start(url string) error
	Stop()
	Started() bool
	Url() string
}

type ScreencastManager interface {
	Enabled() bool
	Started() bool
	Image() ([]byte, error)
}

type StreamSinkManager interface {
	ID() string
	Codec() codec.RTPCodec
	Bitrate() int

	AddListener(listener *func(sample Sample)) error
	RemoveListener(listener *func(sample Sample)) error
	MoveListenerTo(listener *func(sample Sample), targetStream StreamSinkManager) error

	ListenersCount() int
	Started() bool

	CreatePipeline() error
	DestroyPipeline()
}

type StreamSrcManager interface {
	Codec() codec.RTPCodec

	Start(codec codec.RTPCodec) error
	Stop()
	Push(bytes []byte)

	Started() bool
}

type CaptureManager interface {
	Start()
	Shutdown() error

	GetBitrateFromVideoID(videoID string) (int, error)

	Broadcast() BroadcastManager
	Screencast() ScreencastManager
	Audio() StreamSinkManager
	Video() BucketsManager

	Webcam() StreamSrcManager
	Microphone() StreamSrcManager
}

type VideoConfig struct {
	Width       string            `mapstructure:"width"`        // expression
	Height      string            `mapstructure:"height"`       // expression
	Fps         string            `mapstructure:"fps"`          // expression
	Bitrate     int               `mapstructure:"bitrate"`      // pipeline bitrate
	GstPrefix   string            `mapstructure:"gst_prefix"`   // pipeline prefix, starts with !
	GstEncoder  string            `mapstructure:"gst_encoder"`  // gst encoder name
	GstParams   map[string]string `mapstructure:"gst_params"`   // map of expressions
	GstSuffix   string            `mapstructure:"gst_suffix"`   // pipeline suffix, starts with !
	GstPipeline string            `mapstructure:"gst_pipeline"` // whole pipeline as a string
}

func (config *VideoConfig) GetPipeline(screen ScreenSize) (string, error) {
	values := map[string]any{
		"width":  screen.Width,
		"height": screen.Height,
		"fps":    screen.Rate,
	}

	language := []gval.Language{
		gval.Function("round", func(args ...any) (any, error) {
			return (int)(math.Round(args[0].(float64))), nil
		}),
	}

	// get fps pipeline
	fpsPipeline := "! video/x-raw ! videoconvert ! queue"
	if config.Fps != "" {
		eval, err := gval.Full(language...).NewEvaluable(config.Fps)
		if err != nil {
			return "", err
		}

		val, err := eval.EvalFloat64(context.Background(), values)
		if err != nil {
			return "", err
		}

		fpsPipeline = fmt.Sprintf("! capsfilter caps=video/x-raw,framerate=%d/100 name=framerate ! videoconvert ! queue", int(val*100))
	}

	// get scale pipeline
	scalePipeline := ""
	if config.Width != "" && config.Height != "" {
		eval, err := gval.Full(language...).NewEvaluable(config.Width)
		if err != nil {
			return "", err
		}

		w, err := eval.EvalInt(context.Background(), values)
		if err != nil {
			return "", err
		}

		eval, err = gval.Full(language...).NewEvaluable(config.Height)
		if err != nil {
			return "", err
		}

		h, err := eval.EvalInt(context.Background(), values)
		if err != nil {
			return "", err
		}

		scalePipeline = fmt.Sprintf("! videoscale ! capsfilter caps=video/x-raw,width=%d,height=%d name=resolution ! queue", w, h)
	}

	// get encoder pipeline
	encPipeline := fmt.Sprintf("! %s name=encoder", config.GstEncoder)
	for key, expr := range config.GstParams {
		if expr == "" {
			continue
		}

		val, err := gval.Evaluate(expr, values, language...)
		if err != nil {
			return "", err
		}

		if val != nil {
			encPipeline += fmt.Sprintf(" %s=%v", key, val)
		} else {
			encPipeline += fmt.Sprintf(" %s=%s", key, expr)
		}
	}

	// join strings with space
	return strings.Join([]string{
		fpsPipeline,
		scalePipeline,
		config.GstPrefix,
		encPipeline,
		config.GstSuffix,
	}[:], " "), nil
}

func (config *VideoConfig) GetBitrateFn(getScreen func() ScreenSize) func() (int, error) {
	return func() (int, error) {
		if config.Bitrate > 0 {
			return config.Bitrate, nil
		}

		screen := getScreen()

		values := map[string]any{
			"width":  screen.Width,
			"height": screen.Height,
			"fps":    screen.Rate,
		}

		language := []gval.Language{
			gval.Function("round", func(args ...any) (any, error) {
				return (int)(math.Round(args[0].(float64))), nil
			}),
		}

		// TOOD: do not read target-bitrate from pipeline, but only from config.

		// TODO: This is only for vp8.
		expr, ok := config.GstParams["target-bitrate"]
		if !ok {
			// TODO: This is only for h264.
			expr, ok = config.GstParams["bitrate"]
			if !ok {
				return 0, fmt.Errorf("bitrate not found")
			}
		}

		bitrate, err := gval.Evaluate(expr, values, language...)
		if err != nil {
			return 0, fmt.Errorf("failed to evaluate bitrate: %w", err)
		}

		var bitrateInt int
		switch val := bitrate.(type) {
		case int:
			bitrateInt = val
		case float64:
			bitrateInt = (int)(val)
		default:
			return 0, fmt.Errorf("bitrate is not int or float64")
		}

		return bitrateInt, nil
	}
}
