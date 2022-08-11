package cli

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"os"
	"errors"
	"runtime"
	. "bitbucket.org/germelcar/campred/common"
)

type Cli struct {

	InFile     	string
	OutFile    	string
	NumSeqs    	int
	NumThreads 	int
	NumSend		int
	Algos      	uint8
	Keep       	bool
	Verbose    	bool
}

var cli Cli

func init() {
	flag.BoolVarP(&cli.Verbose, "verbose", "v", false, "Show extra information")
	flag.BoolVarP(&cli.Keep,"keep", "k", true, "Keep intermediate file")
	flag.StringVarP(&cli.InFile, "input", "i", "", "Input filename")
	flag.StringVarP(&cli.OutFile, "output", "o", "", "Output filename")
	flag.IntVarP(&cli.NumSeqs, "nseqs", "n", 1,
		"Split in multiple parts of `n` parts each one")
	flag.IntVarP(&cli.NumThreads, "threads", "t", runtime.NumCPU(), "Number of threads")
	flag.IntVarP(&cli.NumSend, "send", "s", MAXNUMTRIESSEND,
		fmt.Sprintf("%s %d)", "Max number of times to send each request (max.", MAXNUMTRIESSEND))

	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: %s FLAGS ARGUMENTS\n\n", os.Args[0])
		fmt.Fprintf(os.Stdout, "%s v%s\n\n", "CAMPRED - CAMP AMP PREDiction", VERSION)
		fmt.Fprintf(os.Stdout, "%s:\n", "Flags")
		usages := flag.CommandLine.FlagUsages()
		fmt.Fprint(os.Stdout, usages)

		fmt.Fprintf(os.Stdout, "\n%s:\n", "Arguments")
		fmt.Fprintf(os.Stdout, "  %-21s %s\n", "svm", "Support Vector Machine")
		fmt.Fprintf(os.Stdout, "  %-21s %s\n", "ann", "Artificial Neural Network")
		fmt.Fprintf(os.Stdout, "  %-21s %s\n", "rf" , "Random Forest")
		fmt.Fprintf(os.Stdout, "  %-21s %s\n", "da", "Disciminant Analysis")
		fmt.Fprintf(os.Stdout, "  %-21s %s\n", "all", "All the algorithms above")
	}

	flag.ErrHelp = errors.New("")
	flag.Parse()
}

func NewCli() (Cli) {
	return cli
}

func (c *Cli) Parse() (bool, error) {

	if len(os.Args) == 1 {
		return false, errors.New("No arguments detected. Provide at least one")
	}

	for _, a := range flag.Args() {

		switch a {

		case "all":
			c.Algos |= SVM | ANN | RF | DA

		case "svm":
			c.Algos |= SVM

		case "ann":
			c.Algos |= ANN

		case "rf":
			c.Algos |= RF

		case "da":
			c.Algos |= DA

		default:
			WarningLog.Printf("%s: %s", "Unrecognized algorithm argument", a)

		}
	}

	if c.Algos == 0 {
		return false, errors.New("No valid algorithms provided. Provide at lest one")
	}


	// Check the number of parts
	if c.NumSeqs < 1 {
		WarningLog.Printf("%s. %s", "Number of seqs to split less than 1", "Set to 1")
		c.NumSeqs = 1
	}

	// Check the number of threads
	if c.NumThreads <= 0 || c.NumThreads > runtime.NumCPU() {
		WarningLog.Printf("Invalid number of threads: %d. Set to %d", c.NumThreads, runtime.NumCPU())
		c.NumThreads = runtime.NumCPU()
	}

	// Check the number of times to resend a request
	if c.NumSend <= 0 || c.NumSend > MAXNUMTRIESSEND {
		WarningLog.Printf("Invalid number for resend a request: %d. Set to %d", c.NumSend, MAXNUMTRIESSEND)
		c.NumSend = MAXNUMTRIESSEND
	}

	// Check if input file name exists
	if c.InFile == "" {
		return false, errors.New("input filename is empty")
	}

	_, err := os.Stat(c.InFile)

	if err != nil {
		if os.IsNotExist(err) {
			return false, errors.New(fmt.Sprintf("%s: %s", "Unable to find file", c.InFile))
		}
	}

	// Check write permissions
	fh, err := os.Create(c.OutFile)

	if err != nil {
		return false, err
	}

	fh.Close()
	err = os.Remove(c.OutFile)

	if err != nil {
		return false, err
	}

	return true, nil // all OK
}

func (c *Cli) PrintOptions() {

	fmt.Println("---------------------------- CONFIGURATION ----------------------------")
	fmt.Printf("Input file: %s\n", cli.InFile)
	fmt.Printf("Output file: %s\n", cli.OutFile)
	fmt.Printf("Number of sequences to split: %d\n", c.NumSeqs)
	fmt.Printf("Number of threads: %d\n", c.NumThreads)
	fmt.Print("Algorithms: ")

	if c.Algos & (SVM | ANN | RF | DA) == SVM | ANN | RF | DA {
		fmt.Println("all (svm, ann, rf & da)")

	} else {
		if c.Algos & SVM  == SVM{
			fmt.Print("svm ")
		}

		if c.Algos & ANN == ANN {
			fmt.Print("ann ")
		}

		if c.Algos & RF == RF {
			fmt.Print("rf ")
		}

		if c.Algos & DA == DA {
			fmt.Print("da ")
		}

		fmt.Println("")
	}

	fmt.Printf("Verbose: %v\n", c.Verbose)
	fmt.Printf("Keep files: %v\n", c.Keep)
	fmt.Printf("%s\n\n", "-----------------------------------------------------------------------")

}
