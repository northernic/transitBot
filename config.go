package main

type Config struct {
	BotToken     string           `yaml:"botToken"`
	FromGroups   map[string]int64 `yaml:"fromGroups"`
	HandleGroups map[string]int64 `yaml:"handleGroups"`
}
