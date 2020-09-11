package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/mrxinu/gosolar"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const version = "0.1.1"

// ComplianceResult holds information about a single compliance result.
type ComplianceResult struct {
	NodeID      string `json:"NodeID"`
	NodeCaption string `json:"NodeCaption"`
	XMLResults  string `json:"XMLResults"`
	RuleName    string `json:"RuleName"`
}

// Violation holds a single node/interface pair.
type Violation struct {
	NodeName         string
	RuleName         string
	ConfigBlockMatch string
	PatternText      string
	InViolation      string
	FoundLineNumber  string
}

func main() {
	// define the command-line options
	pflag.BoolP("version", "v", false, "print version")
	pflag.StringP("file", "f", "", "input JSON file")
	pflag.StringP("rule", "r", "", "name of rule to target (default: all rules)")
	pflag.Parse()

	// bind the command-line options
	viper.BindPFlags(pflag.CommandLine)

	if viper.GetBool("version") {
		fmt.Fprintln(os.Stderr, "version "+version)
		os.Exit(0)
	}

	// set the logging level and format
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{
		DisableColors:    true,
		FullTimestamp:    true,
		QuoteEmptyFields: true,
	})

	if err := process(); err != nil {
		log.Fatal(err)
	}
}

func process() error {
	complianceResults, err := getComplianceResults()
	if err != nil {
		return errors.Wrap(err, "failed to get compliance results")
	}

	violations, err := getViolations(complianceResults)
	if err != nil {
		return errors.Wrap(err, "failed to get violations")
	}

	fmt.Printf("found %d violations\n", len(violations))

	if err := writeViolations(violations); err != nil {
		return errors.Wrap(err, "failed to save violations")
	}

	return nil
}

func getComplianceResults() ([]*ComplianceResult, error) {
	inputFileName := viper.GetString("file")

	var content []byte
	if inputFileName != "" {
		var err error
		content, err = ioutil.ReadFile(inputFileName)
		if err != nil {
			return nil, errors.Wrap(err, "could not open file")
		}
	} else {
		hostname := os.Getenv("solarwinds_hostname")
		username := os.Getenv("solarwinds_username")
		password := os.Getenv("solarwinds_password")

		if hostname == "" {
			fmt.Fprintln(os.Stderr, "You must provide a hostname (env: solarwinds_hostname)")
			os.Exit(1)
		}

		if username == "" {
			fmt.Fprintln(os.Stderr, "You must provide a username (env: solarwinds_username)")
			os.Exit(1)
		}

		if password == "" {
			fmt.Fprintln(os.Stderr, "You must provide a password (env: solarwinds_password)")
			os.Exit(1)
		}

		client := gosolar.NewClient(hostname, username, password, true)

		query := `
			SELECT DISTINCT
				NCM_Nodes.NodeID
				, NCM_Nodes.NodeCaption
				, CacheResults.XMLResults
				, CacheResults.RuleName
			FROM Cirrus.Nodes AS NCM_Nodes
			INNER JOIN Cirrus.ConfigArchive AS ConfigArchive ON NCM_Nodes.NodeID = ConfigArchive.NodeID
			INNER JOIN Cirrus.PolicyCacheResults AS CacheResults ON ConfigArchive.ConfigID = CacheResults.ConfigID
			INNER JOIN (
				SELECT
					ConfigArchive.NodeID
					, MAX(ConfigArchive.DownloadTime) AS MostRecentDownload
				FROM Cirrus.ConfigArchive AS ConfigArchive
				WHERE ConfigArchive.ConfigType = 'Running'
				GROUP BY ConfigArchive.NodeID
			) tbl1 ON ConfigArchive.NodeID = tbl1.NodeID AND ConfigArchive.DownloadTime = tbl1.MostRecentDownload
			WHERE NCM_Nodes.MachineType LIKE '%36xx%'
			AND CacheResults.IsViolation = 'True'	
		`

		var err error
		if viper.GetString("rule") != "" {
			query += " AND CacheResults.RuleName = @ruleName"

			parameters := map[string]string{
				"ruleName": viper.GetString("rule"),
			}

			content, err = client.Query(query, parameters)
		} else {
			content, err = client.Query(query, nil)
		}

		if err != nil {
			return nil, errors.Wrap(err, "failed to query")
		}
	}

	// there are &#xD; (carriage returns) at the end of each of the interface
	// names because they were pulled from the configuration that way
	content = bytes.ReplaceAll(content, []byte("&#xD;"), []byte(""))

	var complianceResults []*ComplianceResult
	if err := json.Unmarshal(content, &complianceResults); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal")
	}

	return complianceResults, nil
}

func getViolations(complianceResults []*ComplianceResult) ([]*Violation, error) {
	var violations []*Violation

	type ComplianceDetail struct {
		ConfigBlocks []struct {
			ConfigBlockMatch string `xml:"L,attr"`
			PatternBlock     struct {
				Patterns []struct {
					FoundMatch  string `xml:"FM,attr"`
					PatternText string `xml:"PT,attr"`
					FoundLine   struct {
						FoundLineMatch  string `xml:"FL,attr"`
						FoundLineNumber string `xml:"FLN,attr"`
					} `xml:"L"`
				} `xml:"P"`
			} `xml:"Ps"`
		} `xml:"CB"`
	}

	// iterate over all the device compliance results and then their interfaces
	for _, c := range complianceResults {
		var complianceDetail ComplianceDetail
		if err := xml.Unmarshal([]byte(c.XMLResults), &complianceDetail); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal")
		}

		for _, configBlock := range complianceDetail.ConfigBlocks {
			for _, pattern := range configBlock.PatternBlock.Patterns {
				var foundLineNumber string
				if pattern.FoundMatch == "True" {
					foundLineNumber = pattern.FoundLine.FoundLineNumber
				}

				violations = append(violations, &Violation{
					NodeName:         c.NodeCaption,
					RuleName:         c.RuleName,
					ConfigBlockMatch: configBlock.ConfigBlockMatch,
					PatternText:      pattern.PatternText,
					InViolation:      pattern.FoundMatch,
					FoundLineNumber:  foundLineNumber,
				})
			}
		}
	}

	return violations, nil
}

func writeViolations(violations []*Violation) error {
	now := time.Now()
	dateString := now.Format("200601021504") // <year><month><day><hour><min>

	outputFilename := fmt.Sprintf("ICE-Compliance-Report_%s.csv", dateString)

	// find the executable path
	ex, err := os.Executable()
	if err != nil {
		return errors.Wrap(err, "failed to get executable")
	}
	exePath := filepath.Dir(ex)

	outputFilePath := filepath.Join(exePath, outputFilename)

	csvFile, err := os.Create(outputFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to create output file")
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	defer csvWriter.Flush()

	// write header row
	csvWriter.Write([]string{
		"Node Name",
		"Rule Name",
		"Config Block Match",
		"Pattern Text",
		"In Violation",
		"Line Number",
	})

	for _, v := range violations {
		row := []string{
			v.NodeName,
			v.RuleName,
			v.ConfigBlockMatch,
			v.PatternText,
			v.InViolation,
			v.FoundLineNumber,
		}

		if err := csvWriter.Write(row); err != nil {
			return errors.Wrap(err, "failed to write to CSV file")
		}
	}

	return nil
}
