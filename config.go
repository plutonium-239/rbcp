package main

import (
	"errors"
	"io/fs"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var (
	fixedWidth = lipgloss.NewStyle().Width(8)
	helpStyle  lipgloss.Style
	impStyle   lipgloss.Style
	pathStyle  lipgloss.Style
	errorStyle lipgloss.Style
)

type Config struct {
	UseNerdFontArrow bool
	ShowProgress bool
	Theme Theme
}

type Theme struct {
	ColorNeutral string
	ColorPrimary string
	ColorSecondary string
	ColorError string
	ColorProgress [2]string
}

func defaultConfig() Config {
	return Config{
		UseNerdFontArrow: false,
		ShowProgress: true,
		Theme: Theme{
			ColorNeutral: "#626262",
			ColorPrimary: "#5956E0",
			ColorSecondary: "#ADBDFF",
			ColorError: "#DA4167",
			ColorProgress: [2]string{"#5956E0", "#EE6FF8"},
		},
	}
}

func GetConfig() Config {
	// TODO: config file for preferences?
	// All because this does not give $HOME/.config on windows whereas most devs do have it set to ~
	configDir, _ := os.UserConfigDir()
	if envHome := os.Getenv("HOME"); envHome != "" {
		configDir = envHome + "/.config"
	}
	conf := defaultConfig()
	log.Debugf("Default config is %v", conf) 
	_, err := toml.DecodeFile(configDir + "/rbcp.toml", &conf)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// f, err := os.Create(configDir + "/rbcp.toml")
			// if err != nil {
			// 	log.Errorf("Could not create config file, continuing with defaults.")
			// }
		} else {
			log.Errorf("could not decode config file: %w", err)
		}
	}
	log.Infof("Decoded config is %v", conf)
	setThemeColors(conf.Theme)
	return conf
}

func setThemeColors(t Theme) {
	helpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(t.ColorNeutral))
	impStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(t.ColorPrimary)).Bold(true)
	pathStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(t.ColorSecondary)).Italic(true)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(t.ColorError)).Bold(true)
}