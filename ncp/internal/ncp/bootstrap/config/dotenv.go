package config

import (
	"bufio"
	"os"
	"strings"
)

func LoadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}

		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		v = strings.Trim(v, `"'`)

		if k == "" {
			continue
		}

		if _, exists := os.LookupEnv(k); exists {
			continue
		}

		if strings.HasPrefix(v, "~/") || v == "~" {
			home, _ := os.UserHomeDir()
			if home != "" {
				if v == "~" {
					v = home
				} else {
					v = home + v[1:]
				}
			}
		}

		_ = os.Setenv(k, v)
	}
}
