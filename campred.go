package main

import (
	"os"
	"bitbucket.org/germelcar/campred/cli"
	. "bitbucket.org/germelcar/campred/common"
	"bitbucket.org/germelcar/campred/bio"
	"bitbucket.org/germelcar/campred/util"
	"fmt"
	"time"
)



func main() {

	mCli := cli.NewCli()
	ok, err := mCli.Parse()

	if !ok {
		ErrorLog.Println(err)
		os.Exit(1)
	}

	mCli.PrintOptions()
	start := time.Now()
	StatusLog.Printf(":::%s:::", "Splitting sequences")
	var preds map[int]struct{}

	// If NumSeqs == 1 means that the entire file will be processed at one. No split needed
	if mCli.NumSeqs == 1 {

		if mCli.Verbose {
			InfoLog.Println("Processing the entire file")
		}

		fFile, err := bio.StatFasta(mCli.InFile)

		if err != nil {
			ErrorLog.Println(err)
			os.Exit(1)
		}

		if mCli.Verbose {
			InfoLog.Printf("Read %d sequences\n", fFile.NumSeqs)
		}

		fmt.Println("")
		StatusLog.Printf(":::%s:::", "Predicting")
		preds = util.Predict([]bio.FastaFile{fFile}, mCli.NumSend, mCli.Algos, mCli.Keep, mCli.Verbose)

		// If there was an error writting the sequences, then inform it, otherwise, append the last written file
		// to the slice of files created from the splitting
		if err != nil {
			WarningLog.Println(err)
		}

	} else {
		fFiles, tot, err := bio.SplitFasta(mCli.InFile, mCli.OutFile, mCli.NumSeqs, mCli.Verbose)

		if err != nil && len(fFiles) == 0 {
			ErrorLog.Println(err)
			os.Exit(1)
		}

		if mCli.Verbose {
			InfoLog.Printf("Read %d sequences. Splitted in %d files of %d sequences (at most) each one\n",
				tot, len(fFiles), mCli.NumSeqs)
		}

		fmt.Println("")
		StatusLog.Printf(":::%s:::", "Predicting")
		preds = util.Predict(fFiles, mCli.NumSend, mCli.Algos, mCli.Keep, mCli.Verbose)

	}

	totPreds := len(preds)

	if totPreds == 0 {
		StatusLog.Println(":::Extracting sequences:::")
		InfoLog.Println("No sequences to extract.")

	} else {

		StatusLog.Printf(":::%s %d %s:::", "Extracting", len(preds), "predicteds as AMP")
		err = bio.ExtractSeqs(mCli.InFile, mCli.OutFile, preds, mCli.Verbose)

		if err != nil {
			ErrorLog.Println(err)
			os.Exit(1)
		}
	}

	fmt.Printf("\n\nElapsed: %s\n", time.Since(start))
}

