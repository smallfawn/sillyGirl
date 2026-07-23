package core

import cron "github.com/robfig/cron/v3"

var CRON *cron.Cron

func init() {
	CRON = cron.New(cron.WithParser(cron.NewParser(
		cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)))
	CRON.Start()
}
