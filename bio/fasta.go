package bio

import (
	"sync"
	"bufio"
	"io"
	"strings"
	"fmt"
	"os"
	. "bitbucket.org/germelcar/campred/common"
	"errors"
)

type FastaSeq struct {

	ID		string
	Seq		string
}

type FastaFile struct {

	FileName	string
	NumSeqs		int
}

const NEWLINE = byte('\n')

func (f *FastaSeq) Len() int {
	return len(f.Seq)
}

func (f FastaSeq) Write(wrt *bufio.Writer) error {

	_, err := wrt.WriteString(fmt.Sprintf("%s\n%s\n", f.ID, f.Seq))

	if err != nil {
		return err
	}

	return nil
}

func WriteSeqs(outFile string, fseqs []FastaSeq) {

	fout, err := os.Create(outFile)
	wrt := bufio.NewWriter(fout)

	defer fout.Close()
	defer wrt.Flush()

	if err != nil {
		ErrorLog.Println(err)
		os.Exit(1)
	}

	for _, f := range fseqs {
		err = f.Write(wrt)

		if err != nil {
			ErrorLog.Printf("Error while writting splitted sequences to file %s: %s\n", outFile, err)
			os.Exit(1)
		}
	}
}

func SplitFasta(inFile, outFile string, numSeqs int, verbose bool) ([]FastaFile, int, error) {

	fin, err := os.Open(inFile)

	if err != nil {
		return nil, 0, err
	}

	defer fin.Close()

	// File reader and channel for keeping those "splitted" files that were successful splitted
	rdr := bufio.NewReader(fin)
	outWritten := make(chan *FastaFile, 10)

	// Total output/splitted files
	// Fasta sequences to split
	// Splitted filenames
	var totOutFiles 	= 1
	var totSeqs 		int
	fseqs := 			[]FastaSeq{}
	outFastaFiles :=	[]FastaFile{}

	// Waigroup for the goroutines that will write the sequences
	// id, seq, and filename of the splitted sequences
	// number of line reading
	var wg 		sync.WaitGroup
	var wgC		sync.WaitGroup
	var id 		string
	var seq 	string
	var fname	string
	numLine := 1

	// One time arrived here, means that the splitted file was written successful
	// Keep waiting and processing (appending to the slice) all the splitted filenames.
	wgC.Add(1)
	go func() {
		defer wgC.Done()

		for ff := range outWritten {
			outFastaFiles = append(outFastaFiles, *ff)
		}

	}()

	READSEQS:
	for {

		// Read up to (and including) the new line character (\n)
		line, err := rdr.ReadBytes(NEWLINE)

		// If there was an error and if the error is EOF, break loop, otherwise, return the error
		if err != nil {
			if err == io.EOF {
				break READSEQS
			}

			return nil, 0, errors.New(fmt.Sprintf("Error while reading line %d: %s", numLine, err))
		}

		// Convert slice of bytes to string before of the new line character and remove spaces on the left side but
		// including the possible ">" character from the line as the ID of the fasta sequence
		sline := strings.TrimLeft(string(line[ : len(line) - 1]), " ")

		if sline[0] == '>' {

			spaceIdx := strings.Index(sline, " ")

			if len(id) > 0 {
				fseqs = append(fseqs, FastaSeq{id, seq})
				seq = ""
				id = ""

				if len(fseqs) == numSeqs {

					wg.Add(1)
					fname = outFile + "_" + fmt.Sprint(totOutFiles) + ".fasta"
					totOutFiles++
					totSeqs += numSeqs

					// Send to write the sequences in the file. If everything OK, then
					// "inform" to the channel that such file was written, otherwise,
					// put a warning on the screen
					go func (ofile string, fs []FastaSeq) {
						defer wg.Done()

						totFs := len(fs)
						WriteSeqs(ofile, fs)
						outWritten <- &FastaFile{ofile, totFs}

						if verbose {
							InfoLog.Printf("Splitted %d sequences in file: %s\n", totFs, ofile)
						}

					}(fname, fseqs)

					fseqs = []FastaSeq{}

				} // End of if len(fseqs)...

			} // End of if len(id) > 0...

			// If "spaceIdx == 1", then the ID sequence has no spaces and then
			// include all the line, otherwise, keep up to a character before the space
			if spaceIdx == -1 {
				id += sline
			} else {
				id += sline[ : spaceIdx]
			}

		} else { // if-else sline[0] == '>'...
			seq += sline
		}

		numLine++

	} // End of for... reading lines

	// Wait all goroutines to finish and close the channel
	// only the goroutines are "using" the channel. One time the goroutines have finished
	// the channel is not used anymore
	wg.Wait()
	close(outWritten)
	wgC.Wait()

	// Add the last sequence
	if seq != "" && id != "" {
		fseqs = append(fseqs, FastaSeq{id, seq})
	}

	// Writte the last set of sequences. If "numSeqs" is equal to the total number of sequences
	// in the file, then, fseqs contains all the sequences of the file. In this case, write the left sequences
	fname = outFile + "_" + fmt.Sprint(totOutFiles) + ".fasta"
	totFs := len(fseqs)
	totSeqs += totFs
	WriteSeqs(fname, fseqs)
	outFastaFiles = append(outFastaFiles, FastaFile{fname, totFs})

	if verbose {
		InfoLog.Printf("Splitted %d sequences in file: %s", totFs, fname)
	}

	return outFastaFiles, totSeqs, nil // all OK
}

func StatFasta(inFile string) (FastaFile, error) {

	fin, err := os.Open(inFile)

	if err != nil {
		return FastaFile{}, err
	}

	defer fin.Close()
	rdr := bufio.NewReader(fin)
	var id	string
	var seq string
	numLine := 1
	numSeqs := 0

	READSEQS:
	for {
		line, err := rdr.ReadBytes(NEWLINE)

		if err != nil {
			if err == io.EOF {
				break READSEQS
			}

			return FastaFile{}, errors.New(fmt.Sprintf("Error while reading line %d: %s", numLine, err))
		}

		sline := strings.TrimLeft(string(line[ : len(line) - 1]), " ")

		if sline[0] == '>' {

			if len(id) > 0 {
				numSeqs++
				id = ""

			}

			id = "."

		} else {
			seq = "."
		}

		numLine++
	}

	if seq != "" && id != "" {
		numSeqs++
	}

	return FastaFile{FileName:inFile, NumSeqs:numSeqs}, nil
}

func ExtractSeqs(inFile, outFile string, seqs map[int]struct{}, verbose bool) error {

	totSeqs := len(seqs)
	fin, err := os.Open(inFile)
	defer fin.Close()

	if err != nil {
		return err
	}

	fout, err := os.Create(outFile)

	if err != nil {
		return err
	}

	rdr := bufio.NewReader(fin)
	wrt := bufio.NewWriter(fout)
	defer wrt.Flush()

	totWritten := 0
	numSeq := 1
	numLine := 1
	var id	string
	var seq string


	READSEQS:
	for {

		line, err := rdr.ReadBytes(NEWLINE)

		if err != nil {
			if err == io.EOF {
				break READSEQS
			}

			return errors.New(fmt.Sprintf("Error while reading line %d: %s", numLine, err))
		}

		sline := strings.TrimLeft(string(line[ : len(line) - 1]), " ")

		if sline[0] == '>' {

			spaceIdx := strings.Index(sline, " ")

			if len(id) > 0 {

				fs := FastaSeq{id, seq}
				_, ok := seqs[numSeq]
				id = ""
				seq = ""

				if ok {

					err := fs.Write(wrt)

					if err != nil {
						return errors.New(fmt.Sprintf("%s: %s", "Unable to extract sequence", err))
					} else {
						totWritten++

						if verbose {
							fmt.Printf("[%d/%d] sequences extracted\r", totWritten, totSeqs)
						}
					}

				}

				numSeq++

			} // End of len(id) > 0...

			if spaceIdx == -1 {
				id += sline
			} else {
				id += sline[ : spaceIdx]
			}

		} else { // End of if-else sline[0] == '>'...
			seq += sline
		}

		numLine++

	}

	if seq != "" && id != "" {

		_, ok := seqs[numSeq]

		if ok {
			fs := FastaSeq{id, seq}
			err := fs.Write(wrt)

			if err != nil {
				return errors.New(fmt.Sprintf("%s: %s", "Unable to extract sequence", err))
			} else {
				totWritten++

				if verbose {
					fmt.Printf("[%d/%d] sequences extracted\r", totWritten, totSeqs)
				}
			}

		}

	}

	if totWritten != totSeqs {
		return errors.New(fmt.Sprintf("%d sequences extracted from %d", totWritten, totSeqs))
	}

	return nil
}