package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/jmespath/jp/Godeps/_workspace/src/github.com/codegangsta/cli"
	"github.com/jmespath/jp/Godeps/_workspace/src/github.com/jmespath/go-jmespath"
)

const version = "0.1.3"

func main() {
	app := cli.NewApp()
	app.Name = "jp"
	app.Version = version
	app.Usage = "jp [<options>] <expression>"
	app.Author = ""
	app.Email = ""
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "filename, f",
			Usage: "Read input JSON from a file instead of stdin.",
		},
		cli.StringFlag{
			Name:  "expr-file, e",
			Usage: "Read JMESPath expression from the specified file.",
		},
		cli.BoolFlag{
			Name:   "unquoted, u",
			Usage:  "If the final result is a string, it will be printed without quotes.",
			EnvVar: "JP_UNQUOTED",
		},
		cli.BoolFlag{
			Name:  "ast",
			Usage: "Only print the AST of the parsed expression.  Do not rely on this output, only useful for debugging purposes.",
		},
		cli.BoolFlag{
			Name:  "no-null",
			Usage: "Omit null outputs.",
		},
		cli.BoolFlag{
			Name: "pretty",
			Usage: "Output indented JSON",
		},
		cli.IntFlag{
			Name: "buffer-size",
			Usage: "Size of input buffer",
		},
	}
	app.Action = runMainAndExit

	app.Run(os.Args)
}

func runMainAndExit(c *cli.Context) {
	os.Exit(runMain(c))
}

func errMsg(msg string, a ...interface{}) int {
	fmt.Fprintf(os.Stderr, msg, a...)
	fmt.Fprintln(os.Stderr)
	return 1
}

func runMain(c *cli.Context) int {
	var expression string
	if c.String("expr-file") != "" {
		byteExpr, err := ioutil.ReadFile(c.String("expr-file"))
		expression = string(byteExpr)
		if err != nil {
			return errMsg("Error opening expression file: %s", err)
		}
	} else {
		if len(c.Args()) == 0 {
			return errMsg("Must provide at least one argument.")
		}
		expression = c.Args()[0]
	}
	if c.Bool("ast") {
		parser := jmespath.NewParser()
		parsed, err := parser.Parse(expression)
		if err != nil {
			if syntaxError, ok := err.(jmespath.SyntaxError); ok {
				return errMsg("%s\n%s\n",
					syntaxError,
					syntaxError.HighlightLocation())
			}
			return errMsg("%s", err)
		}
		fmt.Println("")
		fmt.Printf("%s\n", parsed)
		return 0
	}
	var jsonParser *json.Decoder
	if c.String("filename") != "" {
		f, err := os.Open(c.String("filename"))
		if err != nil {
			return errMsg("Error opening input file: %s", err)
		}
		jsonParser = json.NewDecoder(f)

	} else {
		jsonParser = json.NewDecoder(os.Stdin)
	}

	bufferSize := c.Int("buffer-size")
	if bufferSize == 0 {
		bufferSize = 1
	}
	inputs := make(chan interface{}, 1)
	go func() {
		for jsonParser.More() {
			var input interface{}
			if err := jsonParser.Decode(&input); err != nil {
				panic(fmt.Errorf("Error parsing input json: %s\n", err))
			}
			inputs <- input
		}
		close(inputs)
	}()
	for input := range inputs {
		result, err := jmespath.Search(expression, input)
		if err != nil {
			if syntaxError, ok := err.(jmespath.SyntaxError); ok {
				return errMsg("%s\n%s\n",
					syntaxError,
					syntaxError.HighlightLocation())
			}
			return errMsg("Error evaluating JMESPath expression: %s", err)
		}
		if result != nil || !c.Bool("no-null") {
			converted, isString := result.(string)
			if c.Bool("unquoted") && isString {
				os.Stdout.WriteString(converted)
			} else {
				var toJSON []byte
				var err error
				if c.Bool("pretty") {
					toJSON, err = json.MarshalIndent(result, "", "  ")
				} else {
					toJSON, err = json.Marshal(result)
				}
				if err != nil {
					errMsg("Error marshalling result to JSON: %s\n", err)
					return 3
				}
				os.Stdout.Write(toJSON)
			}
			os.Stdout.WriteString("\n")
		}
	}
	return 0
}
