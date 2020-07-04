//
//
// elastic-mapping-generator is a tool to automate the creation of json files that 
// has elastic:mappings comments above the definition of the struct.
//
// by default, it will generate one json file called `<srcfilename>_mappings.json`
// at the same directory as the source file.

// For example, given ths snippet in users.go

// 		 package data
// 		 
// 		 import (
// 		 	"time"
// 		 
// 		 	sub_data "elastic-mapping-generator/data/sub-data"
// 		 )
// 		 
// 		 // user mapping
// 		 // elastic:mappings
// 		 type User struct {
// 		 	Posts   sub_data.Posts `json:"posts"`
// 		 	Created time.Time
// 		 	User2
// 		 	Name string `json:"name"` // 这是name
// 		 	Pass string `json:"pass" es:"analyzer:ik_smart"`
// 		 }
// 		 
// 		 type User2 struct {
// 		 	Pass2 string `json:"pass2"`
// 		 }
//
// running this command
// in the same directory will create the file users_mappings.json
//
// you can see details in data directory
//
// of course, just as the tool stringer in go tools package,
// you can use go generate, like this:
//      //go:generate elastic-mapping-generator ./data/users.go
//
// it still has so many to do, but not now

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	suffix = flag.String("suffix", "", "suffix of file name; default _mappings.json \texample: srcdir/<srcfilename>_mappings.json")
)

// Usage is a replacement usage function for the flags package
func Usage() {
	fmt.Fprint(os.Stderr, "Usage of elastic-mapping-generator:\n")
	fmt.Fprintf(os.Stderr, "\t elastic-mapping-generator files...\n")
	flag.PrintDefaults()
}

func main() {
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[elastic-mapping-generator] ")
	flag.Usage = Usage
	flag.Parse()

	// parse fileSuffix
	var fileSuffix = "mappings"
	if len(*suffix) > 0 {
		fileSuffix = *suffix
	}
	var files []string
	args := flag.Args()
	if len(args) == 0 {
		Usage()
		return
	} else {
		files = args
	}

	var wg sync.WaitGroup
	for _, fileName := range files {
		wg.Add(1)
		f, _ := filepath.Abs(fileName)
		go func() {
			defer wg.Done()
			var generator = new(Generator)
			generator.SetSuffix(fileSuffix)
			generator.Generate(f)
		}()
	}
	wg.Wait()
}

// isDirectory reports whether the named file is a directory.
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// elastic-mapping-generator is a tool to automate creation of struct
type Generator struct {
	suffix  string
	imports map[string]string
}

func (g *Generator) SetSuffix(suffix string) {
	g.suffix = suffix
}

// type map from go to elastic
var typeMap = map[string]interface{}{
	"int64":   "long",
	"int":     "integer",
	"short":   "integer",
	"byte":    "byte",
	"float64": "double",
	"float":   "float",
	"Time":    "date",
	"string":  "text",
}

type Properties map[string]interface{}

func (p Properties) Merge(other Properties) Properties {
	for k, v := range other {
		p[k] = v
	}
	return p
}
func (g *Generator) exploderBaseDir(fileName string) {
	if len(g.imports) == 0 {
		return
	}
	var somePath string
	var prefix string
	for _, path := range g.imports {
		somePath = path
		idx := strings.Index(fileName, strings.Split(somePath, "/")[0])
		if idx > 0 {
			prefix = fileName[:idx]
			break
		}
	}
	for k, path := range g.imports {
		if isDirectory( prefix + path){
			g.imports[k] = prefix + path
		}else{
			delete(g.imports, k)
		}
	}
}
func (g *Generator) Generate(fileName string) {
	var fileSet = token.NewFileSet()
	fileTree, err := parser.ParseFile(fileSet, fileName, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}
	g.imports = map[string]string{}
	for _, im := range fileTree.Imports {
		path, _ := strconv.Unquote(im.Path.Value)
		var name string
		if im.Name != nil {
			name = im.Name.Name
		} else {
			pathSlice := strings.Split(path, "/")
			name = pathSlice[len(pathSlice)-1]
		}
		g.imports[name] = path
	}
	g.exploderBaseDir(fileName)

	properties := Properties{}
	for _, decl := range fileTree.Decls {
		switch x1 := decl.(type) {
		case *ast.GenDecl:
			var isMappings bool
			if x1.Doc != nil {
				for _, c := range x1.Doc.List {
					matched, _ := regexp.MatchString("// elastic:mappings", c.Text)
					if matched {
						isMappings = true
						break
					}
				}
			}
			if !isMappings {
				continue
			}
			spc := x1.Specs[0]
			switch x2 := spc.(type) {
			case *ast.TypeSpec:
				properties = g.ParseDecl(x2, nil)
				if properties != nil {
					break
				}
			}
		}
	}
	var outputName string
	pieces := strings.Split(fileName, ".")
	pieces[len(pieces)-2] = pieces[len(pieces)-2] + "_" + g.suffix + ".json"
	outputName = strings.Join(pieces[:len(pieces)-1], ".")
	vv, err := json.MarshalIndent(map[string]interface{}{
		"mappings": map[string]interface{}{
			"properties": properties,
		},
	}, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.OpenFile(outputName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	// _, _ = f.WriteString(fmt.Sprintf("// Code generated by \"elastic-mapping-generator %s\"; DO NOT EDIT.\n", strings.Join(os.Args[1:], " ")))
	_, _ = f.Write(vv)
	if err != nil {
		log.Fatal(err)
	}
}
func (g Generator) ParseDecl(typeSpec *ast.TypeSpec, fileTree map[string]*ast.Package) Properties {
	properties := Properties{}
	tpe, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return nil
	}
	// 解析字段
	for _, f := range tpe.Fields.List {
		fieldFileTree := fileTree
		attributes := Properties{}
		var ftypeSpec *ast.TypeSpec
		switch ftype := f.Type.(type) {
		case *ast.Ident:
			if ftype.Obj != nil {
				ftypeSpec, _ = ftype.Obj.Decl.(*ast.TypeSpec)
			}
		case *ast.SelectorExpr:
			pv, _ := g.imports[ftype.X.(*ast.Ident).Name]
			var err error
			if pv != "" {
				fieldFileTree, err = parser.ParseDir(token.NewFileSet(), pv, nil, parser.ParseComments)
				if err != nil {
					log.Fatal(err)
				}
				ftypeSpec = g.searchSpec(fieldFileTree, ftype.Sel.Name)
			}
		case *ast.ArrayType:
			ftypeSpec = nil
		}

		var name string
		// use field name as default
		if f.Names != nil {
			name = f.Names[0].Name
		}
		var esAttributes Properties
		if f.Tag != nil {
			// user name in json if exists
			tag := reflect.StructTag(f.Tag.Value[1:len(f.Tag.Value)])
			jsonTag := tag.Get("json")
			// skip this field as json says
			if jsonTag == "-" {
				continue
			} else {
				jsonTagList := strings.Split(jsonTag, ",")
				if len(jsonTagList) > 0 {
					name = jsonTagList[0]
				}
			}
			esAttributes = g.parseEsTag(tag)
		}
		// properties from related object of certain field
		extProperties := Properties{}

		// situation of anonymous
		if name == "" && ftypeSpec == nil {
			if _, ok := f.Type.(*ast.Ident); ok {
				ftypeSpec = g.searchSpec(fileTree, f.Type.(*ast.Ident).Name)
			}
		}
		if ftypeSpec != nil {
			extProperties = g.ParseDecl(ftypeSpec, fieldFileTree)
		}
		if name == "" {
			properties.Merge(extProperties)
		} else if ftypeSpec != nil {
			properties[name] = Properties{
				"properties": extProperties,
			}
		} else {
			attributes["type"] = g.parseType(f)
			attributes.Merge(esAttributes)
			properties[name] = attributes
		}
	}
	return properties
}

func (g Generator) parseEsTag(tag reflect.StructTag) Properties {
	var attributes = Properties{}
	// parse es tag
	esTag := tag.Get("es")
	if esTag != "" {
		attrStrList := strings.Split(esTag, ",")
		for _, attrStr := range attrStrList {
			attrs := strings.Split(attrStr, ":")
			if len(attrs) < 2 {
				log.Fatal("es tag parse failed")
			}
			attributes[attrs[0]] = strings.Join(attrs[1:], ":")
		}
	}
	return attributes
}

// just parse the most popular type of go
// but you can define it in es tag, the default will be overwritten
func (g Generator) parseType(f *ast.Field) interface{} {
	var name string
	switch tpe2 := f.Type.(type) {
	case *ast.Ident:
		name = tpe2.Name
	case *ast.SelectorExpr:
		name = tpe2.Sel.Name
	case *ast.ArrayType:
		return "array"
	}
	if t, ok := typeMap[name]; ok {
		return t
	}
	return "unknown"
}

func (g *Generator) searchSpec(tree map[string]*ast.Package, tpe string) *ast.TypeSpec {
	for _, pkg := range tree {
		for _, f := range pkg.Files {
			if f.Decls == nil {
				continue
			}
			for _, decl := range f.Decls {
				if _, ok := decl.(*ast.GenDecl); ok {
					for _, spec := range decl.(*ast.GenDecl).Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							if typeSpec.Name.Name == tpe {
								return typeSpec
							}
						}
					}
				}
			}
		}
	}
	return nil
}
