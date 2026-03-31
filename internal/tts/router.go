package tts

import (
	"context"
	"fmt"
	"strings"
)

const (
	ProviderPiper      = "piper"
	ProviderElevenLabs = "elevenlabs"
	ProviderAuto       = "auto"
)

type ProviderRouter struct {
	piper          Client
	elevenLabs     Client
	providerSource func(context.Context) string
	onCreditFallbk func(context.Context)
}

func NewProviderRouter(
	piper Client,
	elevenLabs Client,
	providerSource func(context.Context) string,
	onCreditFallback func(context.Context),
) *ProviderRouter {
	return &ProviderRouter{
		piper:          piper,
		elevenLabs:     elevenLabs,
		providerSource: providerSource,
		onCreditFallbk: onCreditFallback,
	}
}

func normalizeProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case ProviderElevenLabs:
		return ProviderElevenLabs
	case ProviderAuto:
		return ProviderAuto
	default:
		return ProviderPiper
	}
}

func (r *ProviderRouter) currentProvider(ctx context.Context) string {
	if r.providerSource == nil {
		return ProviderPiper
	}
	return normalizeProvider(r.providerSource(ctx))
}

func (r *ProviderRouter) Synthesize(ctx context.Context, text, voice, language, outDir string) (string, error) {
	switch r.currentProvider(ctx) {
	case ProviderPiper:
		return r.piper.Synthesize(ctx, text, voice, language, outDir)
	case ProviderElevenLabs, ProviderAuto:
		if r.elevenLabs == nil {
			return r.piper.Synthesize(ctx, text, voice, language, outDir)
		}
		path, err := r.elevenLabs.Synthesize(ctx, text, voice, language, outDir)
		if err == nil {
			return path, nil
		}
		if isElevenCreditsError(err) {
			if r.onCreditFallbk != nil {
				r.onCreditFallbk(ctx)
			}
			fallbackPath, fallbackErr := r.piper.Synthesize(ctx, text, voice, language, outDir)
			if fallbackErr == nil {
				return fallbackPath, nil
			}
			return "", fmt.Errorf("elevenlabs credits exhausted (%v), fallback tts also failed: %w", err, fallbackErr)
		}
		if r.currentProvider(ctx) == ProviderAuto {
			return r.piper.Synthesize(ctx, text, voice, language, outDir)
		}
		return "", err
	default:
		return r.piper.Synthesize(ctx, text, voice, language, outDir)
	}
}

func (r *ProviderRouter) ListVoices(ctx context.Context) ([]Voice, error) {
	switch r.currentProvider(ctx) {
	case ProviderPiper:
		return r.piper.ListVoices(ctx)
	case ProviderElevenLabs:
		if r.elevenLabs == nil {
			return r.piper.ListVoices(ctx)
		}
		return r.elevenLabs.ListVoices(ctx)
	case ProviderAuto:
		if r.elevenLabs == nil {
			return r.piper.ListVoices(ctx)
		}
		voices, err := r.elevenLabs.ListVoices(ctx)
		if err == nil {
			return voices, nil
		}
		return r.piper.ListVoices(ctx)
	default:
		return r.piper.ListVoices(ctx)
	}
}

func (r *ProviderRouter) Preview(ctx context.Context, text, voice, language string) ([]byte, error) {
	switch r.currentProvider(ctx) {
	case ProviderPiper:
		return r.piper.Preview(ctx, text, voice, language)
	case ProviderElevenLabs, ProviderAuto:
		if r.elevenLabs == nil {
			return r.piper.Preview(ctx, text, voice, language)
		}
		audio, err := r.elevenLabs.Preview(ctx, text, voice, language)
		if err == nil {
			return audio, nil
		}
		if isElevenCreditsError(err) || r.currentProvider(ctx) == ProviderAuto {
			if isElevenCreditsError(err) && r.onCreditFallbk != nil {
				r.onCreditFallbk(ctx)
			}
			return r.piper.Preview(ctx, text, voice, language)
		}
		return nil, err
	default:
		return r.piper.Preview(ctx, text, voice, language)
	}
}

func isElevenCreditsError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	keywords := []string{
		"insufficient",
		"credit",
		"quota",
		"payment required",
		"quota_exceeded",
		"too_many_concurrent_requests",
	}
	for _, k := range keywords {
		if strings.Contains(message, k) {
			return true
		}
	}
	return false
}
