# elastic-mapping-generator
elastic-mapping-generator is a tool to automate the creation of json files that 
has `elastic:mappings` comments above the definition of the struct.

by default, it will generate one json file called `<srcfilename>_mappings.json`
at the same directory as the source file.

## install
```bash
go install github.com/darcyaf/elastic-mapping-generator
```
## usage
```bash
Usage of elastic-mapping-generator:
         elastic-mapping-generator files...
  -suffix string
        suffix of file name; default _mappings.json     example: srcdir/<srcfilename>_mappings.json
```
## example
For example, given ths snippet in users.go
```go
 package data
 
 import (
    "time"
 
    sub_data "elastic-mapping-generator/data/sub-data"
 )
 
 //go:generate elastic-mapping-generator ./users.go
 //elastic:mappings
 type User struct {
    Posts   sub_data.Posts `json:"posts"`
    Created time.Time
    User2
    Name string `json:"name"` 这是name
    Pass string `json:"pass" es:"analyzer:ik_smart"`
 }
 
 type User2 struct {
    Pass2 string `json:"pass2"`
 }
```
it will generate file
```
{
    "mappings": {
        "properties": {
            "Created": {
                "type": "date"
            },
            "name": {
                "type": "text"
            },
            "pass": {
                "analyzer": "ik_smart",
                "type": "text"
            },
            "pass2": {
                "type": "text"
            },
            "posts": {
                "properties": {
                    "Id": {
                        "type": "long"
                    }
                }
            }
        }
    }
}
```

## faq
1. parse failed, got empty json?
    in this program, I only import direct packages in the file header, if you use over two level packages, it may cannot parse.
    it's better to add `import _ "other_package"` so it can search.