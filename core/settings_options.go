package core

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/smallfawn/sillyGirl/utils"
)

type settingOptionNormalizer func(string) (string, error)

func settingOptions(storageKey string, defaults []string, selected string, normalize settingOptionNormalizer) []string {
	rows := []string{}
	appendOne := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if normalize != nil {
			normalized, err := normalize(value)
			if err != nil {
				return
			}
			value = normalized
		}
		if value == "" || Contains(rows, value) {
			return
		}
		rows = append(rows, value)
	}
	for _, value := range defaults {
		appendOne(value)
	}
	for _, value := range customSettingOptions(storageKey) {
		appendOne(value)
	}
	appendOne(selected)
	return rows
}

func addSettingOption(storageKey string, raw string, normalize settingOptionNormalizer) (string, error) {
	value := strings.TrimSpace(raw)
	if normalize != nil {
		normalized, err := normalize(value)
		if err != nil {
			return "", err
		}
		value = normalized
	}
	if value == "" {
		return "", errors.New("地址不能为空")
	}
	rows := customSettingOptions(storageKey)
	if !Contains(rows, value) {
		rows = append(rows, value)
		saveCustomSettingOptions(storageKey, rows)
	}
	return value, nil
}

func removeSettingOption(storageKey string, raw string, defaults []string, normalize settingOptionNormalizer) (string, error) {
	value := strings.TrimSpace(raw)
	if normalize != nil {
		normalized, err := normalize(value)
		if err != nil {
			return "", err
		}
		value = normalized
	}
	if value == "" {
		return "", errors.New("地址不能为空")
	}
	for _, item := range defaults {
		if normalize != nil {
			normalized, err := normalize(item)
			if err == nil {
				item = normalized
			}
		}
		if strings.TrimSpace(item) == value {
			return "", errors.New("默认地址不能删除")
		}
	}
	rows := customSettingOptions(storageKey)
	next := rows[:0]
	for _, item := range rows {
		if item != value {
			next = append(next, item)
		}
	}
	saveCustomSettingOptions(storageKey, next)
	return value, nil
}

func customSettingOptions(storageKey string) []string {
	raw := strings.TrimSpace(sillyGirl.GetString(storageKey))
	if raw == "" {
		return nil
	}
	rows := []string{}
	_ = json.Unmarshal([]byte(raw), &rows)
	return rows
}

func saveCustomSettingOptions(storageKey string, rows []string) {
	if len(rows) == 0 {
		sillyGirl.Set(storageKey, "")
		return
	}
	sillyGirl.Set(storageKey, string(utils.JsonMarshal(rows)))
}
