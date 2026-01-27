package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
)

type OAuthAuthorizeResp struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func Login(baseURL string, provider string) error {
	u := fmt.Sprintf("%s/oauth/authorize/%s", baseURL, provider)
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var ar OAuthAuthorizeResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return err
	}
	fmt.Println("Opening browser for OAuth login...")
	if err := openBrowser(ar.AuthURL); err != nil {
		fmt.Println("Open this URL manually:", ar.AuthURL)
	}
	fmt.Println("After finishing login, run: devorch login-status", provider)
	return nil
}

func LoginStatus(baseURL string, provider string) error {
	u := fmt.Sprintf("%s/oauth/status/%s", baseURL, provider)
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var v map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&v)
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
	return nil
}

func Logout(baseURL string, provider string) error {
	u := fmt.Sprintf("%s/oauth/logout/%s", baseURL, provider)
	req, _ := http.NewRequest("POST", u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println("Logged out:", provider)
	return nil
}
