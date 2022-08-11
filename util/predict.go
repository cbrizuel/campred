package util


import (
	"sync"
	"bitbucket.org/germelcar/campred/bio"
	"bytes"
	"mime/multipart"
	"os"
	"io"
	"net/http"
	. "bitbucket.org/germelcar/campred/common"
	"bufio"
	"github.com/PuerkitoBio/goquery"
	"strings"
	"strconv"
	"io/ioutil"
	"fmt"
	"errors"
)

type predRequest struct {
				bio.FastaFile
	NumSent		int
	IdxFile		int
	PrevSeqs	int
}

type predResponse struct {
	buff	[]byte
			*predRequest
}


func writeResponse(presp *predResponse, wg *sync.WaitGroup, verbose bool) {
	defer wg.Done()

	if verbose {
		InfoLog.Printf("Writting response for %s (%s)",
			presp.FileName, presp.FileName + ".camp")
	}

	newName := presp.FileName + ".camp"
	ofile, err := os.Create(newName)
	defer ofile.Close()

	if err != nil {
		WarningLog.Printf("Error while reading response body for file %s: %s", presp.FileName, err)
		return
	}

	w := bufio.NewWriter(ofile)
	defer w.Flush()
	_, err = w.Write(presp.buff)

	if err != nil {
		WarningLog.Printf("Error while writting response body for file %s: %s", presp.FileName, err)
	}

}

func addAlgorithms(algos uint8, mp *multipart.Writer) error {

	// Adding the algorithms to use for prediction
	// Iterate over the flags while adding each algorithm
	// and removing it from the flags
	for algos != 0 {

		fw, err := mp.CreateFormField("algo[]")

		if err != nil {
			return err
		}

		if algos & SVM == SVM {

			_, err = fw.Write([]byte("svm"))

			if err != nil {
				return err
			}

			algos &^= SVM
			continue
		}

		if algos & ANN == ANN {

			_, err = fw.Write([]byte("ann"))

			if err != nil {
				return err
			}

			algos &^= ANN
			continue
		}

		if algos & RF == RF {

			_, err = fw.Write([]byte("rf"))

			if err != nil {
				return err
			}

			algos &^= RF
			continue
		}

		if algos & DA == DA {

			_, err = fw.Write([]byte("da"))

			if err != nil {
				return err
			}

			algos &^= DA
			continue
		}

	}

	return nil
}

func sendFile(preq *predRequest, algos uint8, numSend, totFiles, totAlgos int, keep, verbose bool,
		wgSend, wgResp *sync.WaitGroup, limitCh chan bool, finishCh chan *predRequest, predsCh chan int) {

	defer func() {
		<-limitCh
	}()
	defer wgSend.Done()


	SEND:

		preq.NumSent++

		if preq.NumSent > numSend {
			finishCh <- preq
			return

		} else {

			if verbose {

				if preq.NumSent > 1 {
					InfoLog.Printf(">>Resending %s by %d of %d times (file %d of %d)",
						preq.FileName, preq.NumSent, numSend, preq.IdxFile, totFiles)
				} else {
					InfoLog.Printf("Sending %s by %d of %d times (file %d of %d)",
						preq.FileName, preq.NumSent, numSend, preq.IdxFile, totFiles)
				}
			}
		}


		var b bytes.Buffer
		mp := multipart.NewWriter(&b)

		f, err := os.Open(preq.FileName)
		defer f.Close()

		if err != nil {
			WarningLog.Printf("Error sending file %s: %s", preq.FileName, err)
			goto SEND
		}

		fw, err := mp.CreateFormFile("userfile", preq.FileName)

		if err != nil {
			WarningLog.Printf("Error creating HTTP POST Form for file %s: %s", preq.FileName, err)
			goto SEND
		}

		_, err = io.Copy(fw, f)

		if err != nil {
			WarningLog.Println(err)
			goto SEND
		}

		// Add algorithms
		addAlgorithms(algos, mp)
		mp.Close()

		req, err := http.NewRequest("POST", CAMPREDURL, &b)

		if err != nil {
			WarningLog.Printf("Error sending POST request for file %s: %s", preq.FileName, err)
			goto SEND
		}

		req.Header.Set("Content-Type", mp.FormDataContentType())
		httpClient := http.Client{Timeout: REQUESTTIMEOUT}
		res, err := httpClient.Do(req)

		if err != nil {
			WarningLog.Printf("Error with HTTP response for file %s: %s", preq.FileName, err)
			goto SEND
		}

		if res.StatusCode != 200 {
			WarningLog.Printf("Error with HTPP responde for file %s. Response code: %d",
				preq.FileName, res.StatusCode)

			goto SEND
		}

		buff, err := ioutil.ReadAll(res.Body)

		if err != nil {
			WarningLog.Printf("Error while extracting the body response for parsing it for file %s: %s",
				preq.FileName, err)
			goto SEND
		}

		if verbose {

			if preq.NumSent > 1 {
				InfoLog.Printf(">>Reparsing response of %s by %d of %d times (file %d of %d)",
					preq.FileName, preq.NumSent, numSend, preq.IdxFile, totFiles)
			} else {
				InfoLog.Printf("Parsing response of %s by %d of %d times (file %d of %d)",
					preq.FileName, preq.NumSent, numSend, preq.IdxFile, totFiles)
			}
		}

		presp := predResponse{buff: buff, predRequest: preq}
		err = parseResponse(&presp, totAlgos, algos, keep, verbose, wgResp, predsCh)

		if err != nil {
			WarningLog.Println(err)
			goto SEND
		}

		finishCh <- preq
}

func parseResponse(presp *predResponse, totAlgos int, algos uint8, keep, verbose bool,
	wgResp *sync.WaitGroup, predsCh chan int) error {

	rdr := bytes.NewReader(presp.buff)
	results := []string{}
	numRows := 0
	preds := make(map[int]uint8)
	doc, err := goquery.NewDocumentFromReader(rdr)

	if err != nil {
		return err
	}


	doc.Find("table.corner tr td").Each(func(i int, s *goquery.Selection) {


		s.Find("p strong").Each(func(i2 int, s2 *goquery.Selection) {

			// Append all "Results with [ALGORITHM]" where [ALGORITHM] is:
			// SVM, ANN, RF or DA
			if strings.Contains(s2.Text(),"Results") {
				results = append(results, s2.Text())
			}

		})

		var currAlg uint8
		var currAlgStr string

		s.Find("table").Each(func(i2 int, s2 *goquery.Selection) {

			s2.Find("tr").Each(func(i4 int, s3 *goquery.Selection) {

				row := s3.Find("td").Text()

				if row == "" {

					if strings.Contains(results[i2], "Support") {
						currAlg = SVM
						currAlgStr = "SVM"

					} else if strings.Contains(results[i2], "Artificial") {
						currAlg = ANN
						currAlgStr = "ANN"

					} else if strings.Contains(results[i2], "Random") {
						currAlg = RF
						currAlgStr = "RF"

					} else if strings.Contains(results[i2], "Discriminant") {
						currAlg = DA
						currAlgStr = "DA"

					}

				} else {

					row = strings.TrimRight(strings.TrimLeft(row, " "), " ")
					elements := strings.Fields(row)

					// The first row contains the columns: "Seq. ID", "Class" AND/OR "Probability"
					// For the ANN, the "Probability" columns does not exist
					if elements[1] == "AMP" || elements[1] == "NAMP" {

						numRows++

						if elements[1] == "AMP" {

							idx, err := strconv.ParseInt(elements[0], 10, 0)

							if err != nil {
								WarningLog.Printf(
									"Error while parsing to int (%s). File: %s\tSequence: %d\tAlgorithm: %s",
									elements[0], presp.FileName, idx, currAlgStr)
							}

							tid := int(idx) + presp.PrevSeqs
							preds[tid] |= currAlg

						}

					}
				}

			})

		})

	})


	// If the number of predicteds sequences (numRows) is different than number of sequences
	// of the splitted file, that means that the tables were not complete.
	//
	// For example, sometimes the tables comes empty.
	if len(results) != totAlgos || ((presp.NumSeqs * totAlgos) != numRows) {

		return errors.New(fmt.Sprintf(
			"Response body incomplete with %d algorithms' results (%s)",
			len(results), presp.FileName))
	}

	totAmps := 0
	for idx, count := range preds {
		if count == algos {
			totAmps++
			predsCh <- idx
		}
	}

	if verbose {
		InfoLog.Printf("A total of %d sequences (of %d) predicted as AMP (%s)",
			totAmps, presp.NumSeqs, presp.FileName)
	}

	// If keep intermediate files is set, then, write the response
	// with extension ".camp"
	if keep {
		wgResp.Add(1)
		go writeResponse(presp, wgResp, verbose)
	}

	return nil

}

func Predict(files []bio.FastaFile, numSend int, algos uint8, keep, verbose bool) (map[int]struct{}) {

	var wg sync.WaitGroup
	var wgSend sync.WaitGroup
	var wgResp sync.WaitGroup

	var totSeqs int

	// Total request that have failed to be processed
	totFaileds := 0

	// Number of algorithms specified by the user. 4 for all (svm, ann, rf & da)
	totAlgos := NumAlgos(algos)

	// Total files to be processed
	totFiles := len(files)

	// Sequences predicteds as AMP by all the algorithms specified by the user and
	// the requests already processeds
	preds := make(map[int]struct{})
	finishes := []*predRequest{}

	predsCh := make(chan int, 100)
	finishCh := make(chan *predRequest)
	limitCh := make(chan bool, MAXREQUESTS)


	// Add the finishes requests
	//
	// This slice could contains finishes requests mainly by two reasons:
	//
	// 1.- The request was processed and everything was OK
	// 2.- The request was failed to be processed due to the times than the specified by "MAX_REQUESTS"
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < totFiles; i++ {
			f := <- finishCh
			finishes = append(finishes, f)

			if f.NumSent > numSend {
				totFaileds++

				if verbose {
					WarningLog.Printf("File %s (file %d of %d) has failed to be processed",
						f.FileName, f.IdxFile, totFiles)
				}
			}

		}

		close(finishCh)
		close(predsCh)

	}()

	//
	// Add the predicteds sequences by all the algorithms specified by the user
	//
	wg.Add(1)
	go func() {
		defer wg.Done()

		for idx := range predsCh {
			preds[idx] = struct{}{}
		}

	}()

	//
	// Iterate over all splitted fasta files and send them with a limit equals to "MAX_REQUESTS" constant.
	//
	for i, f := range files {

		// The index of the splitted file (n of N). One based.
		idxFile := i + 1

		// Make the request to be send
		preq := &predRequest{
			FastaFile: f,
			NumSent: 0,
			IdxFile: idxFile,
			PrevSeqs: totSeqs,
		}

		// Take a "place" for the number of splitted files to be send concurrently and
		// add one to "wgSend" in order to wait to all goroutines have finished
		limitCh <- true
		wgSend.Add(1)

		// Send the request
		go sendFile(preq, algos, numSend, totFiles, totAlgos, keep, verbose,
			&wgSend, &wgResp, limitCh, finishCh, predsCh)

		// Update the number of sequences before of the splitted fasta file.
		// This is for calculating the real index of the sequences from the results
		// of request. For example:
		//
		// A multifasta of 10 sequences splitted in 2 files of 10 sequences each one,
		// each response will get their sequences numbered from 1 to 10, but the sequences
		// of the second file are 11 to 20 instead of 1 to 10.
		totSeqs += f.NumSeqs

	}

	// Wait all goroutines the send the request, write the response,
	// get the predicteds sequences (by all algorithms specified by the user) and
	// get the requests processes
	wgSend.Wait()
	wgResp.Wait()
	wg.Wait()
	close(limitCh)

	// Report the failed splitted files/requests
	if totFaileds >= 1 {
		var msg string
		if totFaileds > 1 {
			msg = "The following files have failed to be processed:"
		} else {
			msg = "The following file has failed to be processed:"
		}

		WarningLog.Println(msg)

		for _, f := range finishes {
			if f.NumSent > numSend {
				WarningLog.Printf("%s (file %d of %d)", f.FileName, f.IdxFile, totFiles)
			}
		}

	}

	// Reporting the total number of sequences predicteds as AMP by all the algorithms
	// specified by the user
	InfoLog.Printf("A total of %d sequence(s) predicted(s) as AMP by the %d algorithm(s)",
		len(preds), totAlgos)

	return preds
}
