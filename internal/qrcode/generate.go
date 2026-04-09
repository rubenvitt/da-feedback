package qrcode

import (
	"fmt"

	goqrcode "github.com/skip2/go-qrcode"
)

func GeneratePNG(baseURL, slug, secret string, size int) ([]byte, error) {
	url := fmt.Sprintf("%s/f/%s-%s", baseURL, slug, secret)
	png, err := goqrcode.Encode(url, goqrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("generate qr png: %w", err)
	}
	return png, nil
}

func GenerateGlobalPNG(baseURL, globalSecret string, size int) ([]byte, error) {
	url := fmt.Sprintf("%s/f/alle-%s", baseURL, globalSecret)
	png, err := goqrcode.Encode(url, goqrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("generate global qr png: %w", err)
	}
	return png, nil
}

func FeedbackURL(baseURL, slug, secret string) string {
	return fmt.Sprintf("%s/f/%s-%s", baseURL, slug, secret)
}

func GlobalFeedbackURL(baseURL, globalSecret string) string {
	return fmt.Sprintf("%s/f/alle-%s", baseURL, globalSecret)
}
