package web

import (
	"net/http"
	"net/url"
)

const (
	flashSuccessCookie = "flash_success"
	flashErrorCookie   = "flash_error"
)

// setFlash sets a flash message cookie
func setFlash(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		Path:     "/",
		MaxAge:   5, // expires in 5 seconds
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// getFlash reads and clears a flash message cookie
func getFlash(w http.ResponseWriter, r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	// Clear the cookie
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	val, _ := url.QueryUnescape(cookie.Value)
	return val
}

// Convenience functions
func setFlashSuccess(w http.ResponseWriter, msg string) {
	setFlash(w, flashSuccessCookie, msg)
}

func setFlashError(w http.ResponseWriter, msg string) {
	setFlash(w, flashErrorCookie, msg)
}
