package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ShylajaDevadiga/go-test-html-report/assets"
	"github.com/spf13/cobra"

	"html/template"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strings"
	"time"
)

type Output struct {
	State string
	Name  string
	Type  string
	Time  float64
}

type GoTestJsonRowData struct {
	Time    time.Time
	Action  string
	Package string
	Test    string
	Output  string
	Elapsed float64
}

type ProcessedTestdata struct {
	TotalTestTime     string
	TestDate          string
	FailedTests       int
	PassedTests       int
	SkippedTests      int
	TestSummary       []TestOverview
	packageDetailsIdx map[string]PackageDetails
}

type PackageDetails struct {
	Name         string
	ElapsedTime  float64
	Status       string
	FailedTests  int
	PassedTests  int
	SkippedTests int
}

type TestDetails struct {
	PackageName string
	Name        string
	ElapsedTime float64
	Status      string
}

type TestOverview struct {
	Test      TestDetails
	TestCases []TestDetails
}

func main() {
	rootCmd := initCommand()
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var fileName string
var packageName string
var packages = make(map[string]string)
var OS string

func initCommand() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "go-test-html-report",
		Short: "go-test-html-report generates a html report of go-test logs",
		RunE: func(cmd *cobra.Command, args []string) (e error) {
			file, _ := cmd.Flags().GetString("file")
			testData := make([]GoTestJsonRowData, 0)
			if file != "" {
				testData = ReadLogsFromFile(file)
			} else {
				testData = ReadLogsFromStdIn()
			}

			processedTestdata := ProcessTestData(testData)
			GenerateHTMLReport(processedTestdata.TotalTestTime,
				processedTestdata.TestDate,
				processedTestdata.FailedTests,
				processedTestdata.PassedTests,
				processedTestdata.SkippedTests,
				processedTestdata.TestSummary,
				processedTestdata.packageDetailsIdx,
			)
			log.Println("Report Generated")
			//launchhtml()
			return nil
		},
	}
	rootCmd.PersistentFlags().StringVarP(
		&fileName,
		"file",
		"f",
		"",
		"set the file of the go test json logs")
	return rootCmd
}

func ReadLogsFromFile(fileName string) []GoTestJsonRowData {
	file, err := os.Open(fileName)
	if err != nil {
		log.Println("error opening file: ", err)
		os.Exit(1)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Println("error closing file: ", err)
			os.Exit(1)
		}
	}()

	// file scanner
	scanner := bufio.NewScanner(file)
	rowData := make([]GoTestJsonRowData, 0)

	for scanner.Scan() {
		row := GoTestJsonRowData{}

		// unmarshall each line to GoTestJsonRowData
		err := json.Unmarshal([]byte(scanner.Text()), &row)
		if err != nil {
			log.Println("error to unmarshall test logs: ", err)
			os.Exit(1)
		}
		if row.Test != "" {
			packageName = row.Test
			packages[row.Test] = row.Action
		}
		rowData = append(rowData, row)
	}

	if err := scanner.Err(); err != nil {
		log.Println("error with file scanner: ", err)
		os.Exit(1)
	}
	return rowData
}

func ReadLogsFromStdIn() []GoTestJsonRowData {
	// stdin scanner
	scanner := bufio.NewScanner(os.Stdin)
	rowData := make([]GoTestJsonRowData, 0)
	for scanner.Scan() {
		row := GoTestJsonRowData{}
		// unmarshall each line to GoTestJsonRowData
		err := json.Unmarshal([]byte(scanner.Text()), &row)
		if err != nil {
			log.Println("error to unmarshall test logs: ", err)
			os.Exit(1)
		}
		rowData = append(rowData, row)
	}
	if err := scanner.Err(); err != nil {
		log.Println("error with stdin scanner: ", err)
		os.Exit(1)
	}
	return rowData
}

// Process data from logs to generate report
func ProcessTestData(rowData []GoTestJsonRowData) ProcessedTestdata {
	testCasesIdx := map[string]TestDetails{}
	packageDetailsIdx := map[string]PackageDetails{}
	passedTests := 0
	failedTests := 0
	skippedTests := 0

	for _, r := range rowData {
		if r.Test != "" {
			packageName = r.Test
			var jsonMap Output
			if strings.Contains(r.Output, "k3s test") {
				output2 := strings.LastIndex(r.Output, "}")
				output2 = output2 + 1
				json.Unmarshal([]byte(strings.TrimSpace(r.Output[:output2])), &jsonMap)
			}
			if strings.Contains(r.Output, "OS") {
				res := strings.Split(r.Output, "/")
				OS = strings.TrimSpace(res[1])
			}
			// if testNameArr not equal 1 then we assume we have a test case
			if len(jsonMap.Name) > 1 {
				if jsonMap.State == "failed" || jsonMap.State == "passed" || jsonMap.State == "skipped" {
					testCasesIdx[r.Test+jsonMap.Name] = TestDetails{
						PackageName: r.Test,
						Name:        jsonMap.Name,
						ElapsedTime: jsonMap.Time / (1000 * 1000 * 1000 * 60),
						Status:      jsonMap.State,
					}
				}
				if jsonMap.State == "failed" {
					failedTests = failedTests + 1
				} else if jsonMap.State == "passed" {
					passedTests = passedTests + 1
				} else if jsonMap.State == "skipped" {
					skippedTests = skippedTests + 1
				}
			}
		} else {
			if r.Action == "fail" || r.Action == "pass" || r.Action == "skip" {
				packageDetailsIdx[packageName] = PackageDetails{
					Name:         packageName,
					ElapsedTime:  r.Elapsed / 60,
					Status:       r.Action,
					FailedTests:  failedTests,
					PassedTests:  passedTests,
					SkippedTests: skippedTests,
				}
			}
			if r.Action == "output" {
				packageDetailsIdx[packageName] = PackageDetails{
					Name:         packageName,
					ElapsedTime:  packageDetailsIdx[r.Test].ElapsedTime,
					Status:       packageDetailsIdx[r.Test].Status,
					FailedTests:  failedTests,
					PassedTests:  passedTests,
					SkippedTests: skippedTests,
				}
			}
		}
	}
	testSummary := make([]TestOverview, 0)
	for key := range packages {
		testCases := make([]TestDetails, 0)
		for _, t2 := range testCasesIdx {
			if t2.PackageName == key {
				testCases = append(testCases, t2)
			}
		}
		testSummary = append(testSummary, TestOverview{
			TestCases: testCases,
		})
	}
	// determine total test time
	totalTestTime := ""
	if rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds() < 60 {
		totalTestTime = fmt.Sprintf("%f s", rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds())
	} else {
		min := int(math.Trunc(rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Seconds() / 60))
		seconds := int(math.Trunc((rowData[len(rowData)-1].Time.Sub(rowData[0].Time).Minutes() - float64(min)) * 60))
		totalTestTime = fmt.Sprintf("%dm:%ds", min, seconds)
	}
	testDate := rowData[0].Time.Format(time.RFC850)
	return ProcessedTestdata{
		TotalTestTime:     totalTestTime,
		TestDate:          testDate,
		FailedTests:       failedTests,
		PassedTests:       passedTests,
		SkippedTests:      skippedTests,
		TestSummary:       testSummary,
		packageDetailsIdx: packageDetailsIdx,
	}
}

// Generate report
func GenerateHTMLReport(totalTestTime, testDate string, failedTests, passedTests int, skippedTests int, testCasesIdx []TestOverview, packageDetailsIdx map[string]PackageDetails) {
	templates := make([]template.HTML, 0)
	for _, v := range packageDetailsIdx {
		htmlString := template.HTML("<div type=\"button\" class=\"collapsible\">\n")
		packageInfoTemplateString := template.HTML("")

		if v.Status != "pass" || v.Status != "fail" || v.Status != "skip" {
			packageInfoTemplateString = "<div>Test Run failed</div>" + "\n" + "<div>Run Time: {{.elapsedTime}}</div> " + "\n"
		}

		packageInfoTemplateString = "<div>{{.packageName}}</div>" + "\n" + "<div>Run Time: {{.elapsedTime}}</div> " + "\n"
		packageInfoTemplate, err := template.New("packageInfoTemplate").Parse(string(packageInfoTemplateString))
		if err != nil {
			log.Println("error parsing package info template", err)
			os.Exit(1)
		}

		var processedPackageTemplate bytes.Buffer
		err = packageInfoTemplate.Execute(&processedPackageTemplate, map[string]string{
			"packageName": v.Name + "_" + OS,
			"elapsedTime": fmt.Sprintf("%.2f", v.ElapsedTime),
		})
		if err != nil {
			log.Println("error applying package info template: ", err)
			os.Exit(1)
		}
		if v.Status == "pass" {
			packageInfoTemplateString = "<div class=\"collapsibleHeading packageCardLayout successBackgroundColor \">" +
				template.HTML(processedPackageTemplate.Bytes()) + "</div>"
		} else if v.Status == "fail" {
			packageInfoTemplateString = "<div class=\"collapsibleHeading packageCardLayout failBackgroundColor \">" +
				template.HTML(processedPackageTemplate.Bytes()) + "</div>"
		} else {
			packageInfoTemplateString = "<div class=\"collapsibleHeading packageCardLayout skipBackgroundColor \">" +
				template.HTML(processedPackageTemplate.Bytes()) + "</div>"
		}

		htmlString = htmlString + "\n" + packageInfoTemplateString

		testInfoTemplateString := template.HTML("")
		for k, pt := range testCasesIdx {
			testHTMLTemplateString := template.HTML("")
			if len(pt.TestCases) == 0 {
				log.Println("Test run failed, Exiting..")
				os.Exit(1)
			}
			//Access testcases for correcponding package
			if pt.TestCases[k].PackageName == v.Name {
				// check if test contains test cases
				if len(pt.TestCases) == 0 {
					// test does not contain test cases
					testHTMLTemplateString = "<div>{{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}s</div>"
					testTemplate, err := template.New("standaloneTests").Parse(string(testHTMLTemplateString))
					if err != nil {
						log.Println("error parsing standalone tests template: ", err)
						os.Exit(1)
					}
					var processedTestTemplate bytes.Buffer
					err = testTemplate.Execute(&processedTestTemplate, map[string]string{
						"testName":    pt.Test.Name,
						"elapsedTime": fmt.Sprintf("%.2f", pt.Test.ElapsedTime),
					})
					if err != nil {
						log.Println("error applying standalone tests template: ", err)
						os.Exit(1)
					}
					if pt.Test.Status == "passed" {
						testHTMLTemplateString = "<div class=\"testCardLayout successBackgroundColor \">" + template.HTML(processedTestTemplate.Bytes()) + "</div>"
					} else if pt.Test.Status == "failed" {
						testHTMLTemplateString = "<div class=\"testCardLayout failBackgroundColor \">" + template.HTML(processedTestTemplate.Bytes()) + "</div>"
					} else {
						testHTMLTemplateString = "<div class=\"testCardLayout skipBackgroundColor \">" + template.HTML(processedTestTemplate.Bytes()) + "</div>"
					}
					testInfoTemplateString = testInfoTemplateString + "\n" + testHTMLTemplateString
					continue
				}

				if pt.Test.Status == "passed" {
					testHTMLTemplateString = "<div type=\"button\" class=\"collapsible \">" +
						"\n" + "<div class=\"collapsibleHeading testCardLayout successBackgroundColor \">" +
						"<div>+ {{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}s</div>" + "\n" +
						"</div>" + "\n" +
						"<div class=\"collapsibleHeadingContent\">"
				} else if pt.Test.Status == "failed" {
					testHTMLTemplateString = "<div type=\"button\" class=\"collapsible \">" +
						"\n" + "<div class=\"collapsibleHeading testCardLayout failBackgroundColor \">" +
						"<div>+ {{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}s</div>" + "\n" +
						"</div>" + "\n" +
						"<div class=\"collapsibleHeadingContent\">"
				} else {
					testHTMLTemplateString = "<div type=\"button\" class=\"collapsible \">" +
						"\n" + "<div class=\"collapsibleHeading testCardLayout skipBackgroundColor \">" +
						"<div>+ {{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}s</div>" + "\n" +
						"</div>" + "\n" +
						"<div class=\"collapsibleHeadingContent\">"
				}

				testTemplate, err := template.New("nonStandaloneTest").Parse(string(testHTMLTemplateString))
				if err != nil {
					log.Println("error parsing non standalone tests template: ", err)
					os.Exit(1)
				}
				var processedTestTemplate bytes.Buffer
				err = testTemplate.Execute(&processedTestTemplate, map[string]string{
					"testName":    pt.Test.Name,
					"elapsedTime": fmt.Sprintf("%f", pt.Test.ElapsedTime),
				})
				if err != nil {
					log.Println("error applying non standalone tests template: ", err)
					os.Exit(1)
				}
				testHTMLTemplateString = template.HTML(processedTestTemplate.Bytes())
				testCaseHTMlTemplateString := template.HTML("")
				for _, tC := range pt.TestCases {
					testCaseHTMlTemplateString = "<div>{{.testName}}</div>" + "\n" + "<div>{{.elapsedTime}}m</div>"
					testCaseTemplate, err := template.New("testCase").Parse(string(testCaseHTMlTemplateString))
					if err != nil {
						log.Println("error parsing test case template: ", err)
						os.Exit(1)
					}

					var processedTestCaseTemplate bytes.Buffer
					err = testCaseTemplate.Execute(&processedTestCaseTemplate, map[string]string{
						"testName":    tC.Name,
						"elapsedTime": fmt.Sprintf("%f", tC.ElapsedTime),
					})
					if err != nil {
						log.Println("error applying test case template: ", err)
						os.Exit(1)
					}
					if tC.Status == "passed" {
						testCaseHTMlTemplateString = "<div class=\"testCardLayout successBackgroundColor \">" + template.HTML(processedTestCaseTemplate.Bytes()) + "</div>"

					} else if tC.Status == "failed" {
						testCaseHTMlTemplateString = "<div  class=\"testCardLayout failBackgroundColor \">" + template.HTML(processedTestCaseTemplate.Bytes()) + "</div>"

					} else {
						testCaseHTMlTemplateString = "<div  class=\"testCardLayout skipBackgroundColor \">" + template.HTML(processedTestCaseTemplate.Bytes()) + "</div>"
					}
					testHTMLTemplateString = testHTMLTemplateString + "\n" + testCaseHTMlTemplateString
				}
				testHTMLTemplateString = testHTMLTemplateString + "\n" + "</div>" + "\n" + "</div>"
				testInfoTemplateString = testInfoTemplateString + "\n" + testHTMLTemplateString
			}
		}
		htmlString = htmlString + "\n" + "<div class=\"collapsibleHeadingContent\">\n" + testInfoTemplateString + "\n" + "</div>"
		htmlString = htmlString + "\n" + "</div>"
		templates = append(templates, htmlString)
	}
	reportTemplate := template.New("report-template.html")

	reportTemplateData, err := assets.Asset("report-template.html")
	if err != nil {
		log.Println("error retrieving report-template.html: ", err)
		os.Exit(1)
	}

	report, err := reportTemplate.Parse(string(reportTemplateData))
	if err != nil {
		log.Println("error parsing report-template.html: ", err)
		os.Exit(1)
	}
	var processedTemplate bytes.Buffer
	type templateData struct {
		HTMLElements  []template.HTML
		FailedTests   int
		PassedTests   int
		SkippedTests  int
		TotalTestTime string
		TestDate      string
	}

	err = report.Execute(&processedTemplate,
		&templateData{
			HTMLElements:  templates,
			FailedTests:   failedTests,
			PassedTests:   passedTests,
			SkippedTests:  skippedTests,
			TotalTestTime: totalTestTime,
			TestDate:      testDate,
		},
	)
	if err != nil {
		log.Println("error applying report-template.html: ", err)
		os.Exit(1)
	}
	// write the whole body at once
	err = ioutil.WriteFile("k3s_"+OS+"_results.html", processedTemplate.Bytes(), 0644)
	if err != nil {
		log.Println("error writing report.html file: ", err)
		os.Exit(1)
	}
}

func launchhtml() {
	http.Handle("/", http.FileServer(http.Dir("./reports")))
	http.ListenAndServe(":80", nil)
}
