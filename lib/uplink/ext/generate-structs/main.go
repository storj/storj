package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/aleitner/gogert"
	"github.com/storj/storj/lib/uplink"
)

type set map[string]struct{}

type structTypeMap map[string]*ast.StructType

var (
	input  = flag.String("input-dir", ".", "input directory")
	output = flag.String("output-dir", "stdout", "output directory")
)

func main() {
	flag.Parse()

	// Check if input path exists
	fi, err := os.Stat(*input)
	if os.IsNotExist(err) {
		log.Fatal(err)
	}

	if !fi.IsDir() {
		log.Fatal(fmt.Errorf("Specified Input Path is not a directory: %s", *output))
	}

	// Validate output path
	var outputFile *os.File
	if *output == "stdout" {
		outputFile = os.Stdout
	} else {
		outputPath := path.Join(*output, "/cstructs.h")

		fi, err := os.Stat(*output)
		if os.IsNotExist(err) {
			log.Fatal(err)
		}

		if !fi.IsDir() {
			log.Fatal(fmt.Errorf("Specified Output Path is not a directory: %s", *output))
		}

		outputFile, err = os.Create(outputPath)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := outputFile.Close(); err != nil {
				log.Fatal(err)
			}
		}()
	}

	w := bufio.NewWriter(outputFile)
	if err := Parse(w, *input); err != nil {
		log.Fatal(err)
	}
	w.Flush()
}

// Parse through a directory for exported go structs to be converted
func Parse(w io.Writer, path string) error {
	var cStructs []*gogert.CStructMeta

	fset := token.NewFileSet()
	astPkgs, err := parser.ParseDir(fset, path, func(info os.FileInfo) bool {
		name := info.Name()
		return !info.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go")
	}, parser.ParseComments)
	if err != nil {
		return err
	}

	// Loop through every file and convert the exported go structs
	for _, pkg := range astPkgs {
		for _, file := range pkg.Files {
			structNames := getStructNames(file)
			structs := findStructs(file, structNames)
			cStructs = append(cStructs, fromGoStructs(structs, fset)...)
		}
	}

	// organize the cstructs
	sort.Slice(cStructs, func(i, j int) bool {
		return len(cStructs[i].DependencyStructNames) < len(cStructs[j].DependencyStructNames)
	})

	for _, cstruct := range cStructs {
		fmt.Fprintf(w, cstruct.String())
	}

	return nil
}

// generateStructRecursivewill generate a cstruct and any cstruct dependencies
func generateStructRecursive(fset *token.FileSet, name string, fields *ast.FieldList) (cstructs []*gogert.CStructMeta, err error) {
	cstructs = []*gogert.CStructMeta{}
	cstruct, err := gogert.NewCStructMeta(name, true)
	if err != nil {
		return cstructs, err
	}
	cstructs = append(cstructs, cstruct)

	for _, field := range fields.List {

		// Get type from ast.Field
		var typeNameBuf bytes.Buffer
		err := printer.Fprint(&typeNameBuf, fset, field.Type)
		if err != nil {
			return cstructs, err
		}

		// Field name
		gotype := typeNameBuf.String()
		fieldName := gotype
		if len(field.Names) > 0 {
			fieldName = field.Names[0].Name
		}

		// Give anonymous structs a name
		fieldGoType := gotype
		if strings.HasPrefix(gotype, "struct {") {
			fieldGoType = fmt.Sprintf("Anonymous struct")
		}

		converter, err := gogert.NewConverter()
		if err != nil {
			return cstructs, err
		}

		reflectType, err := LookupReflectType(gotype)
		if err != nil {
			return cstructs, err
		}

		// Convert type from go to C
		ctype, dependencies := converter.FromGoType(gotype)

		// add all of the field's dependent structs that were found
		cstructs = append(cstructs, dependencies...)

		cstruct.Fields = append(cstruct.Fields, &gogert.Field{
			CType:  ctype,
			Name:   fieldName,
			GoType: fieldGoType,
		})

		// If field itself is a struct make sure we list it as a dependency
		if strings.Contains(ctype, "struct") {
			cstruct.DependencyStructNames = append(cstruct.DependencyStructNames, ctype)
		}

	}

	return cstructs, nil
}

func LookupReflectType(gotype string) (reflectType reflect.Type) {
	types, err := uplink.Troop.Types()
	if err != nil { // something went wrong getting them
		return nil, err
	}

	for _, typ := range types {
		fmt.Println(typ.String())
		if typ.String() == gotype {
			return typ, nil
		}
	}

	return nil, nil
}

func getStructNames(file *ast.File) set {
	// Parse comments for exported structs
	structNames := make(set)
	ast.Inspect(file, func(n ast.Node) bool {
		// collect comments
		c, ok := n.(*ast.CommentGroup)
		if ok {
			re, _ := regexp.Compile(`(?:\n|^)CExport\s+(\w+)`)
			matches := re.FindStringSubmatch(c.Text())
			if len(matches) > 1 {
				structNames[strings.TrimSpace(matches[1])] = struct{}{}
			}
		}
		return true
	})
	return structNames
}

func findStructs(file *ast.File, structNames set) structTypeMap {
	structs := make(structTypeMap)
	ast.Inspect(file, func(n ast.Node) bool {
		t, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		if t.Type == nil {
			return true
		}

		structName := t.Name.Name

		_, ok = structNames[structName]
		if !ok {
			return true
		}

		x, ok := t.Type.(*ast.StructType)
		if !ok {
			return true
		}

		structs[structName] = x

		return true
	})
	return structs
}

func fromGoStructs(structs structTypeMap, fset *token.FileSet) []*gogert.CStructMeta {
	var cStructs []*gogert.CStructMeta
	for name, structType := range structs {
		cstructRecursive, err := generateStructRecursive(fset, name, structType.Fields)
		if err != nil {
			log.Fatal(err)
		}

		cStructs = append(cStructs, cstructRecursive...)
	}
	return cStructs
}
