package chat

import (
	"fmt"
	"image"
	"net/http"
	"regexp"

	_ "image/jpeg"
	_ "image/png"

	"github.com/charmbracelet/lipgloss"
	"github.com/qeesung/image2ascii/convert"
)

func FormatMarkdownImage(text string) string {
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)

	return imageRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := imageRegex.FindStringSubmatch(match)
		alt := submatches[1]
		url := submatches[2]

		// Convertir l'image en ASCII
		asciiArt, err := ImageToAscii(url)
		if err != nil {
			// En cas d'erreur, retourner le format texte comme avant
			return fmt.Sprintf("📷 Image: %s\nURL: %s\nError: %s", alt, url, err)
		}

		imageStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("99")).
			Padding(1)

		return imageStyle.Render(fmt.Sprintf("📷 %s\n%s", alt, asciiArt))
	})
}

// Fonction pour convertir une image en ASCII art
func ImageToAscii(url string) (string, error) {
	// Télécharger l'image
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	// Décoder l'image
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Configurer le convertisseur
	convertOptions := convert.DefaultOptions
	convertOptions.FixedWidth = 60  // Ajuster la largeur selon vos besoins
	convertOptions.FixedHeight = 30 // Ajuster la hauteur selon vos besoins

	// Convertir en ASCII
	converter := convert.NewImageConverter()
	ascii := converter.Image2ASCIIString(img, &convertOptions)

	return ascii, nil
}
