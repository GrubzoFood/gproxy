package main

func main() {
	logger, err := NewLogger()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	logger.Info("starting gproxy")

	run(logger)
}
