package shortcode

import (
	"fmt"
	"time"

	"github.com/goliatone/go-cms/pkg/interfaces"
)

// BuiltInDefinitions returns the core shortcode catalogue shipped with go-cms.
func BuiltInDefinitions() []interfaces.ShortcodeDefinition {
	return []interfaces.ShortcodeDefinition{
		youTubeDefinition(),
		alertDefinition(),
		galleryDefinition(),
		figureDefinition(),
		codeDefinition(),
	}
}

func youTubeDefinition() interfaces.ShortcodeDefinition {
	return interfaces.ShortcodeDefinition{
		Name:        "youtube",
		Version:     "1.0.0",
		Description: "Embeds a responsive YouTube iframe player",
		Category:    "media",
		Icon:        "youtube",
		AllowInner:  false,
		CacheTTL:    time.Hour,
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{
					Name:     "id",
					Type:     interfaces.ShortcodeParamString,
					Required: true,
				},
				{
					Name:    "start",
					Type:    interfaces.ShortcodeParamInt,
					Default: 0,
				},
				{
					Name:    "autoplay",
					Type:    interfaces.ShortcodeParamBool,
					Default: false,
				},
			},
		},
		Template: `{{- $start := printf "?start=%d" .start -}}
<div class="shortcode shortcode--youtube">
  <iframe src="https://www.youtube.com/embed/{{ .id }}{{ if gt .start 0 }}{{ $start }}{{ end }}{{ if .autoplay }}&autoplay=1{{ end }}" title="YouTube video" loading="lazy" allowfullscreen></iframe>
</div>`,
	}
}

func alertDefinition() interfaces.ShortcodeDefinition {
	validateType := func(value any) error {
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("alert type must be string")
		}
		switch str {
		case "info", "success", "warning", "danger":
			return nil
		default:
			return fmt.Errorf("alert type %q not supported", str)
		}
	}

	return interfaces.ShortcodeDefinition{
		Name:        "alert",
		Version:     "1.0.0",
		Description: "Displays contextual alert callouts",
		Category:    "content",
		Icon:        "alert",
		AllowInner:  true,
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{
					Name:     "type",
					Type:     interfaces.ShortcodeParamString,
					Required: true,
					Validate: validateType,
				},
				{
					Name: "title",
					Type: interfaces.ShortcodeParamString,
				},
			},
		},
		Template: `<div class="shortcode shortcode--alert shortcode--alert-{{ .type }}">
  {{ if .title }}<div class="shortcode__title">{{ .title }}</div>{{ end }}
  <div class="shortcode__body">{{ .Inner }}</div>
</div>`,
	}
}

func galleryDefinition() interfaces.ShortcodeDefinition {
	return interfaces.ShortcodeDefinition{
		Name:        "gallery",
		Version:     "1.0.0",
		Description: "Renders an image gallery grid",
		Category:    "media",
		Icon:        "image",
		AllowInner:  false,
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{
					Name:     "images",
					Type:     interfaces.ShortcodeParamArray,
					Required: true,
				},
				{
					Name:    "columns",
					Type:    interfaces.ShortcodeParamInt,
					Default: 3,
				},
			},
		},
		Template: `<div class="shortcode shortcode--gallery columns-{{ .columns }}">
  {{ range .images }}
    <figure class="shortcode__gallery-item">
      <img src="{{ . }}" loading="lazy" />
    </figure>
  {{ end }}
</div>`,
	}
}

func figureDefinition() interfaces.ShortcodeDefinition {
	return interfaces.ShortcodeDefinition{
		Name:        "figure",
		Version:     "1.0.0",
		Description: "Image figure with caption support",
		Category:    "media",
		Icon:        "figure",
		AllowInner:  false,
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{
					Name:     "src",
					Type:     interfaces.ShortcodeParamString,
					Required: true,
				},
				{
					Name:    "alt",
					Type:    interfaces.ShortcodeParamString,
					Default: "",
				},
				{
					Name: "caption",
					Type: interfaces.ShortcodeParamString,
				},
			},
		},
		Template: `<figure class="shortcode shortcode--figure">
  <img src="{{ .src }}" alt="{{ .alt }}" loading="lazy" />
  {{ if .caption }}<figcaption>{{ .caption }}</figcaption>{{ end }}
</figure>`,
	}
}

func codeDefinition() interfaces.ShortcodeDefinition {
	return interfaces.ShortcodeDefinition{
		Name:        "code",
		Version:     "1.0.0",
		Description: "Syntax highlighted code block",
		Category:    "content",
		Icon:        "code",
		AllowInner:  true,
		Schema: interfaces.ShortcodeSchema{
			Params: []interfaces.ShortcodeParam{
				{
					Name:     "lang",
					Type:     interfaces.ShortcodeParamString,
					Required: true,
				},
				{
					Name: "title",
					Type: interfaces.ShortcodeParamString,
				},
				{
					Name:    "line_numbers",
					Type:    interfaces.ShortcodeParamBool,
					Default: true,
				},
			},
		},
		Template: `<div class="shortcode shortcode--code">
  {{ if .title }}<div class="shortcode__code-title">{{ .title }}</div>{{ end }}
  <pre class="shortcode__code-block language-{{ .lang }}{{ if .line_numbers }} shortcode__code-block--lines{{ end }}"><code>{{ .Inner }}</code></pre>
</div>`,
	}
}
