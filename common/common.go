package common

import (
	"log"
	"os"
	"time"
)

var (
	InfoLog   	*log.Logger
	StatusLog 	*log.Logger
	WarningLog	*log.Logger
	ErrorLog  	*log.Logger
)

const (
	SVM		uint8 = 1 << iota
	ANN
	RF
	DA

	CAMPREDURL      = "http://www.camp.bicnirrh.res.in/predict/hii.php"
	REQUESTTIMEOUT  = time.Duration(time.Minute * 10) // Timeout of 10 minutes per request
	MAXNUMTRIESSEND = 10                              // Maximum number of tries per request
	MAXREQUESTS     = 2                              // Maximum number of request concurrently
	VERSION         = "0.1"
)


func init() {

	InfoLog = log.New(os.Stdout, "[INFO] ", log.LstdFlags)

	StatusLog = log.New(os.Stdout, "[STATUS] ", log.LstdFlags)

	WarningLog = log.New(os.Stdout, "[WARNING] ", log.LstdFlags)

	ErrorLog = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime|log.Lshortfile)
}


func NumAlgos(algos uint8) int {
	tot := 0

	if algos & SVM == SVM {
		tot++
	}

	if algos & ANN == ANN {
		tot++
	}

	if algos & RF == RF {
		tot++
	}

	if algos & DA == DA {
		tot++
	}

	return tot
}
