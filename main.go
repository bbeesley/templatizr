package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/urfave/cli"
	"gopkg.in/godo.v2/glob"
	"gopkg.in/yaml.v2"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func pwd() string {
	dir, err := os.Getwd()
	check(err)
	return dir
}

func toAbsolutePath(pointer string) string {
	path, err := filepath.Abs(pointer)
	check(err)
	return path
}

func getFileList(input string, exclude string) []*glob.FileAsset {
	matches, _, err := glob.Glob([]string{input, "!" + exclude})
	check(err)

	return matches
}

func pushGlob(slice []*glob.FileAsset, element *glob.FileAsset) []*glob.FileAsset {
	n := len(slice)
	slice = slice[0 : n+1]
	slice[n] = element
	return slice
}

func getFiles(assets []*glob.FileAsset) []*glob.FileAsset {
	files := assets[0:0]
	for _, asset := range assets {
		if !asset.FileInfo.IsDir() {
			files = pushGlob(files, asset)
		}
	}
	return files
}

// Replacement : pair of strings used to perform a replacement
type Replacement struct {
	Search  string `yaml:"search"`
	Replace string `yaml:"replace"`
}

// ConfigFile : describes a config file for this module
type ConfigFile struct {
	Replacements []Replacement `yaml:"replacements"`
}

func getReplacementConf(replace string) []Replacement {
	replacements := strings.Split(replace, ",")
	result := make([]Replacement, len(replacements))
	for i, r := range replacements {
		pair := strings.Split(r[1:len(r)-1], "/")
		result[i] = Replacement{pair[0], pair[1]}
	}
	return result
}

type errRequiredFlags struct {
	missingFlags []string
}

func (e *errRequiredFlags) Error() string {
	numberOfMissingFlags := len(e.missingFlags)
	if numberOfMissingFlags == 1 {
		return fmt.Sprintf("Required flag %q not set", e.missingFlags[0])
	}
	joinedMissingFlags := strings.Join(e.missingFlags, ", ")
	return fmt.Sprintf("Required flags %q not set", joinedMissingFlags)
}

func main() {
	var input, exclude, output, replace, extension, configPath string
	app := &cli.App{
		Name:  "atl-templatizr",
		Usage: "Convert files to yo templates",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "input",
				Usage:       "glob for the input files",
				Destination: &input,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "exclude",
				Usage:       "glob for files to exclude",
				Destination: &exclude,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "output",
				Usage:       "base path for output files",
				Destination: &output,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "replace",
				Usage:       "replacement patterns, eg /foo/bar/,/baz/bat/ will replace foo with bar and baz with bat",
				Destination: &replace,
			},
			&cli.StringFlag{
				Name:        "config-file",
				Usage:       "replacement patterns, eg /foo/bar/,/baz/bat/ will replace foo with bar and baz with bat",
				Destination: &configPath,
			},
			&cli.StringFlag{
				Name:        "append-extension",
				Usage:       "extra file extension to append to created files",
				Destination: &extension,
				Value:       "",
			},
		},
		Action: func(c *cli.Context) error {
			var replacements []Replacement
			if len(configPath) > 0 {
				c := ConfigFile{}
				yamlFile, readConfigErr := ioutil.ReadFile(toAbsolutePath(configPath))
				check(readConfigErr)
				unmarshalErr := yaml.Unmarshal(yamlFile, &c)
				check(unmarshalErr)
				replacements = c.Replacements
			}
			if len(replace) > 0 {
				replacements = getReplacementConf(replace)
			}
			if len(replacements) == 0 {
				return &errRequiredFlags{missingFlags: []string{"config-file", "replace"}}
			}
			assets := getFileList(input, exclude)
			files := getFiles(assets)
			for _, file := range files {
				mkDirErr := os.MkdirAll(filepath.Join(pwd(), output, filepath.Dir(file.Path)), 0755)
				check(mkDirErr)
				content, err := ioutil.ReadFile(toAbsolutePath(file.Path))
				check(err)
				outfilePath := filepath.Join(pwd(), output, file.Path+extension)
				if utf8.Valid(content) {
					var outFile string = string(content)
					fmt.Println("Applying replacements to " + file.Path)
					for _, replacement := range replacements {
						outFile = strings.ReplaceAll(outFile, replacement.Search, replacement.Replace)
					}
					writeErr := ioutil.WriteFile(outfilePath, []byte(outFile), 0655)
					check(writeErr)
					fmt.Println("Created file with replacements " + outfilePath)
				} else {
					ioutil.WriteFile(outfilePath, content, file.Mode())
					fmt.Println("Copied binary file " + outfilePath)
				}
			}
			return nil
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
