// Data helpers module
package gofakelib

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"log"
	"math"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const prefix = "data/locales"

type LocaleData struct {
	id       uuid.UUID           `json:"id"`
	once     sync.Once           `json:"-"`
	locale   string              `json:"locale"`
	fileName string              `json:"filename"`
	data     map[string][]string `json:"data",omitempty"`
	loaded   bool                `json:"loaded"`
	cacheKey string              `json:"cache_key,omitempty"`
}

type DataLoader struct {
	localeDataMap map[string]*LocaleData
}

type WeightedItem struct {
	Item   string  `json:"item"`
	Weight float64 `json:"weight"`
}

type WeightedArray struct {
	Items []WeightedItem
}

func (w WeightedArray) Validate() (bool, float64) {
	// weights should add to 1.0
	var cumWeight float64 = 0.0

	for _, item := range w.Items {
		cumWeight += item.Weight
	}

	cumWeight = math.Round(cumWeight*100) / 100

	if cumWeight == 1.0 {
		return true, cumWeight
	}
	return false, cumWeight
}

// GetModuleFile returns the full path to a file relative to the module directory
func getModuleFile(relativePath string) (string, error) {
	// Get the current file's directory
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}

	// Get the directory of the current file (helpers.go)
	dir := filepath.Dir(currentFile)

	// Construct the full path to the target file
	fullPath := filepath.Join(dir, relativePath)

	// Verify the file exists (optional)
	_, err := os.Stat(fullPath)
	if err != nil {
		return "", err
	}

	return fullPath, nil
}

func (loader *DataLoader) Init(locale, filePath string) *LocaleData {
	var localeData LocaleData

	log.Printf("Initializing data loader for locale: %s, filePath: %s\n", locale, filePath)

	if len(loader.localeDataMap) == 0 {
		loader.localeDataMap = make(map[string]*LocaleData)
	}

	localeData.fileName = filePath
	localeData.locale = locale
	localeData.id = uuid.New()
	loader.localeDataMap[locale] = &localeData

	return &localeData
}

// Return the specific data pointer for the given locale
func (loader *DataLoader) Get(locale string) *LocaleData {
	var val *LocaleData
	var ok bool

	if val, ok = loader.localeDataMap[locale]; ok {
		return val
	}
	return nil
}

// Fetch value of key
func (l *LocaleData) Get(key string) []string {

	var val []string
	var exists bool

	if val, exists = l.data[key]; !exists {
		log.Printf("error - key doesnt exist %s\n", key)
		return nil
	}
	return val
}

// Load the data for the given locale and fileName
func (l *LocaleData) load() error {

	var err error
	var data []byte

	relPath := path.Join(prefix, l.locale, l.fileName)
	filePath, err := getModuleFile(relPath)
	if err != nil {
		return err
	}

	if _, err = os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return err
	}

	data, err = os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if err = json.Unmarshal(data, &l.data); err != nil {
		return err
	}

	return nil
}

// Do the loading once only for every locale
func (l *LocaleData) Load() error {

	if l.fileName == "" || l.locale == "" {
		return fmt.Errorf("error - loader not initialized")
	}

	var err error

	l.once.Do(func() {
		err = l.load()
		log.Printf("[%s]: loaded locale data once for locale:%s, filename:%s\n", l.id, l.locale, l.fileName)
		l.loaded = true
	})

	return err
}

// Lazy loader wrapper - loads locale data for given locale
// once in a session whenever requested from a function
func (loader *DataLoader) EnsureLoaded(locale string) *LocaleData {
	localeData := loader.Get(locale)
	if localeData == nil {
		// this is a fatal error -we exit
		log.Fatalf("error - locale data not initialized for locale %s", locale)
		return nil
	}

	if !localeData.loaded {
		err := localeData.Load()
		if err != nil {
			log.Printf("error - loading locale data for locale: %s - %v\n", locale, err)
		}
		return localeData
	}

	return localeData
}

// Convert a string to a weighted array
func (l *LocaleData) GetWeightedArray(key, sep string) (*WeightedArray, error) {

	var dataArray WeightedArray

	dataItems := l.Get(key)

	for _, dataItem := range dataItems {
		items := strings.Split(dataItem, sep)
		weight, err := strconv.ParseFloat(items[1], 64)
		if err != nil {
			return nil, err
		}
		dataArray.Items = append(dataArray.Items,
			WeightedItem{Item: items[0], Weight: weight})

	}

	if ok, val := dataArray.Validate(); !ok {
		return nil, fmt.Errorf("weighted array[key: %s] didn't validate - weight %.2f", key, val)
	}

	return &dataArray, nil
}
