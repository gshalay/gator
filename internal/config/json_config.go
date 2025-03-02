package jsonconfig

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	jsonConfigFilename = ".gatorconfig.json"
)

type Config struct {
	DbUrl           string `json:"db_url"`
	CurrentUsername string `json:"current_user_name"`
}

func (c *Config) SetUser(username string) {
	c.CurrentUsername = username

	data, err := json.Marshal(c)
	if err != nil {
		fmt.Printf("%v\n", err)
	}

	path, _ := getConfigJsonPath()

	if path != "" {
		// Now write to the file.
		// Open the file in write-only mode and truncate it (overwrite)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}
		defer file.Close()

		// Write the byte array to the file
		_, err = file.Write(data)
		if err != nil {
			fmt.Println("Error writing to file:", err)
			return
		}
	}
}

func Read() (*Config, error) {
	var c Config
	fileLoc, _ := getConfigJsonPath()

	if fileLoc != "" {
		data, err := os.ReadFile(fileLoc)
		if err != nil {
			return nil, fmt.Errorf("error: %v", err)
		}

		jsonErr := json.Unmarshal(data, &c)
		if jsonErr != nil {
			return nil, jsonErr
		}
	}

	return &c, nil
}

func getConfigJsonPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error reading config json file: unable to fetch home directory")
	} else {
		return home + "/" + jsonConfigFilename, nil
	}
}
