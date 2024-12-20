package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"
)

type Storage interface {
	SaveKeylog(deviceId string, keycode uint16) error
	SaveShortcut(deviceId string, shortcutId int64) error
}

type FileStorage struct {
	fname     string
	keylogs   map[string]map[uint16]int64 // deviceId - keycode - counter
	shortcuts map[string]map[int64]int64  // deviceId - shortcutId - counter
}

type DataFile struct {
	Keylogs   map[string]map[uint16]int64 `json:"keylogs,omitempty"`
	Shortcuts map[string]map[int64]int64  `json:"shortcuts,omitempty"`
}

func newDataFile() DataFile {
	return DataFile{
		Keylogs:   map[string]map[uint16]int64{},
		Shortcuts: map[string]map[int64]int64{},
	}
}

func NewFileStorage(ctx context.Context, fname string) *FileStorage {
	ffs := &FileStorage{
		fname:     fname,
		keylogs:   map[string]map[uint16]int64{},
		shortcuts: map[string]map[int64]int64{},
	}
	// go func(ctx context.Context) {
	// 	ffs.savingInBackground(ctx)
	// }(ctx)
	go ffs.savingInBackground(ctx)
	return ffs
}

func (f *FileStorage) SaveKeylog(deviceId string, keycode uint16) error {
	if _, ok := f.keylogs[deviceId]; !ok {
		f.keylogs[deviceId] = map[uint16]int64{}
	}
	if _, ok := f.keylogs[deviceId][keycode]; !ok {
		f.keylogs[deviceId][keycode] = 0
	}
	f.keylogs[deviceId][keycode] += 1
	return nil
}

func (f *FileStorage) SaveShortcut(deviceId string, shortcutId int64) error {
	if _, ok := f.shortcuts[deviceId]; !ok {
		f.shortcuts[deviceId] = map[int64]int64{}
	}
	if _, ok := f.shortcuts[deviceId][shortcutId]; !ok {
		f.shortcuts[deviceId][shortcutId] = 0
	}
	f.shortcuts[deviceId][shortcutId] += 1
	return nil
}

func (f *FileStorage) prepareDataToSave() (DataFile, error) {
	dataFile, err := getDataFromFile(f.fname)
	if err != nil {
		return dataFile, err
	}
	for kId := range f.keylogs {
		for keycode := range f.keylogs[kId] {
			if _, ok := dataFile.Keylogs[kId][keycode]; ok {
				dataFile.Keylogs[kId][keycode] += f.keylogs[kId][keycode]
				continue
			}
			if _, ok := dataFile.Keylogs[kId]; !ok {
				dataFile.Keylogs[kId] = map[uint16]int64{}
			}
			if _, ok := dataFile.Keylogs[kId][keycode]; !ok {
				dataFile.Keylogs[kId][keycode] = f.keylogs[kId][keycode]
			}
		}
	}
	for kId := range f.shortcuts {
		for scId := range f.shortcuts[kId] {
			if _, ok := dataFile.Shortcuts[kId][scId]; ok {
				dataFile.Shortcuts[kId][scId] += f.shortcuts[kId][scId]
				continue
			}
			if _, ok := dataFile.Shortcuts[kId]; !ok {
				dataFile.Shortcuts[kId] = map[int64]int64{}
			}
			if _, ok := dataFile.Shortcuts[kId][scId]; !ok {
				dataFile.Shortcuts[kId][scId] = f.shortcuts[kId][scId]
			}
		}
	}
	return dataFile, nil
}

func (f *FileStorage) saveToFile() error {
	if len(f.keylogs) == 0 && len(f.shortcuts) == 0 {
		return nil
	}
	start := time.Now()
	data, err := f.prepareDataToSave()
	if err != nil {
		return err
	}
	pb, err := json.Marshal(data)
	if err != nil {
		return err
	}
	err = os.WriteFile(f.fname, pb, 0777)
	if err != nil {
		return err
	}
	slog.Info(fmt.Sprintf("| %s | File %s updated.\n", time.Since(start), f.fname))
	f.keylogs = map[string]map[uint16]int64{}
	f.shortcuts = map[string]map[int64]int64{}
	return nil
}

func (f *FileStorage) savingInBackground(ctx context.Context) {
	for {
		select {
		case <-time.After(30 * time.Second):
			// TODO: And set time to save every 30 s
			f.saveToFile()
		case <-ctx.Done():
			// TODO: gracefull shutdown, make last save .
			slog.Info("Closing file storage...")
			f.saveToFile()
			return
		}
	}
}

func getDataFromFile(fname string) (DataFile, error) {
	dataFile := newDataFile()
	if _, err := os.Stat(fname); errors.Is(err, os.ErrNotExist) {
		slog.Info(fmt.Sprintf("File %s not exist", fname))
		return dataFile, nil
	}

	content, err := os.ReadFile(fname)
	if err != nil {
		slog.Info(fmt.Sprintf("Could not open file %s\n", fname))
		return dataFile, err
	}
	err = json.Unmarshal(content, &dataFile)
	if err != nil {
		slog.Info(fmt.Sprintf("Could not parse file %s, file corrupted\n", fname))
		return dataFile, err
	}
	return dataFile, nil
}
