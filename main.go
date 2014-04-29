package main

import "flag"
import "os"
import "github.com/GarX/go-pac/logger"
import "github.com/GarX/go-pac/worker"

var (
	infile  = flag.String("f", "", "configuration file to input.")
	verbose = flag.Int("v", 2, "give out some output on the screen")
	outfile = flag.String("o", "", "packaged file to output")
)

func main() {
	flag.Parse()
	logger.Level = *verbose
	if outfile == nil || *outfile == "" {
		logger.Debug("Please specified the output file")
		os.Exit(1)
	}
	err := worker.Run(*infile, *outfile)
	if err != nil {
		logger.Debug(err.Error())
	}

}
