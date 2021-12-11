# Dig

A go package that provides a Ruby-like `Hash.dig` mechanism for `map[string]interface{}`, which in YAML terminology is refered to as "Mapping".

## Usage

Dig's Mapping is useful for example to use as the Unmarshal target of arbitary YAML/JSON documents.

### Example
```go
package main

import (
  "fmt"

  "github.com/k0sproject/dig"
  "gopkg.in/yaml.v2"
)

var yamlDoc = []byte(`---
i18n:
  hello:
    se: Hejsan
    fi: Morjens
  world:
    se: Värld
    fi: Maailma
`)

func main() {
  m := dig.Mapping{}
  if err := yaml.Unmarshal(yamlDoc, &m); err != nil {
    panic(err)
  }

  // You can use DigMapping to access a deeply nested map and set values.
  // Any missing Mapping level in between will be created.
  m.DigMapping("i18n", "hello")["es"] = "Hola"
  m.DigMapping("i18n", "world")["es"] = "Mundo"

  langs := []string{"fi", "se", "es"}
  for _, l := range langs {

    // You can use Dig to access a deeply nested value
    greeting := m.Dig("i18n", "hello", l).(string)

    // ..or DigString to avoid having to cast it to string.
    target := m.DigString("i18n", "world", l)

    fmt.Printf("%s, %s!\n", greeting, target)
  }
}
```

Output:

```
Morjens, Maailma!
Hejsan, Värld!
Hola, Mundo!
```
