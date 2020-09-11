package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mrxinu/gosolar"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const version = "0.1.0"

// ComplianceResult holds information about a single compliance result.
type ComplianceResult struct {
	NodeID      string `json:"NodeID"`
	NodeCaption string `json:"NodeCaption"`
	XMLResults  string `json:"XMLResults"`
}

// Violation holds a single node/interface pair.
type Violation struct {
	NodeName      string
	InterfaceName string
}

func main() {
	// define the command-line options
	pflag.BoolP("version", "v", false, "print version")
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

	hostname := os.Getenv("solarwinds_hostname")
	username := os.Getenv("solarwinds_username")
	password := os.Getenv("solarwinds_password")

	client := gosolar.NewClient(hostname, username, password, true)

	if err := process(client); err != nil {
		log.Fatal(err)
	}
}

func process(client *gosolar.Client) error {
	complianceResults, err := getComplianceResults(client)
	if err != nil {
		return errors.Wrap(err, "failed to get compliance results")
	}

	violations, err := getViolations(client, complianceResults)
	if err != nil {
		return errors.Wrap(err, "failed to get violations")
	}

	fmt.Printf("found %d violations\n", len(violations))

	if err := writeViolations(violations); err != nil {
		return errors.Wrap(err, "failed to save violations")
	}

	return nil
}

func getComplianceResults(client *gosolar.Client) ([]*ComplianceResult, error) {
	// TODO: replace this sample file with an actual call to SolarWinds
	// content, err := ioutil.ReadFile("sample_output.json")
	// if err != nil {
	// 	return nil, errors.Wrap(err, "could not open file")
	// }

	query := `
		SELECT DISTINCT
			NCM_Nodes.NodeID
			, NCM_Nodes.NodeCaption
			, CacheResults.XMLResults
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
		AND CacheResults.RuleID = '751cc709-e49c-40fe-9638-0af1627f0499'
		AND CacheResults.IsViolation = 'True'
	`

	res, err := client.Query(query, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query")
	}

	// there are &#xD; (carriage returns) at the end of each of the interface
	// names because they were pulled from the configuration that way
	res = bytes.ReplaceAll(res, []byte("&#xD;"), []byte(""))

	var complianceResults []*ComplianceResult
	if err := json.Unmarshal(res, &complianceResults); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal")
	}

	return complianceResults, nil
}

func getViolations(client *gosolar.Client, complianceResults []*ComplianceResult) ([]*Violation, error) {
	var violations []*Violation

	type ComplianceDetail struct {
		Interfaces []struct {
			InterfaceName string `xml:"L,attr"`
		} `xml:"CB"`
	}

	// iterate over all the device compliance results and then their interfaces
	for _, c := range complianceResults {
		var complianceDetails []ComplianceDetail
		if err := xml.Unmarshal([]byte(c.XMLResults), &complianceDetails); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal")
		}

		for _, cd := range complianceDetails {
			for _, id := range cd.Interfaces {
				violations = append(violations, &Violation{
					NodeName:      c.NodeCaption,
					InterfaceName: id.InterfaceName,
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
		"Interface Name",
	})

	for _, v := range violations {
		row := []string{
			v.NodeName,
			v.InterfaceName,
		}

		if err := csvWriter.Write(row); err != nil {
			return errors.Wrap(err, "failed to write to CSV file")
		}
	}

	return nil
}
