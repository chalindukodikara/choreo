// This program validates whether the exact openAPIV3Schema from a ComponentType CR
// is valid JSON Schema and can be used with a standard JSON Schema parser.
//
// It uses the WHOLE schema as it appears in the CR (with $defs, parameters, and
// envOverrides as siblings), not split-out pieces.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The exact openAPIV3Schema from service-claude-1.yaml, as a JSON Schema document.
// This is what gets stored in etcd as spec.schema.openAPIV3Schema.
const serviceOpenAPIV3Schema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "$defs": {
    "ResourceQuantity": {
      "type": "object",
      "properties": {
        "cpu": {
          "type": "string",
          "default": "100m"
        },
        "memory": {
          "type": "string",
          "default": "256Mi"
        }
      }
    },
    "ResourceRequirements": {
      "type": "object",
      "properties": {
        "requests": {
          "$ref": "#/$defs/ResourceQuantity",
          "default": {}
        },
        "limits": {
          "$ref": "#/$defs/ResourceQuantity",
          "default": {}
        }
      }
    }
  },
  "properties": {
    "parameters": {
      "type": "object",
      "properties": {}
    },
    "envOverrides": {
      "type": "object",
      "properties": {
        "replicas": {
          "type": "integer",
          "default": 1
        },
        "resources": {
          "$ref": "#/$defs/ResourceRequirements",
          "default": {}
        },
        "imagePullPolicy": {
          "type": "string",
          "default": "IfNotPresent",
          "enum": ["Always", "IfNotPresent", "Never"]
        }
      }
    }
  }
}`

// The exact openAPIV3Schema from scheduled-task-claude-1.yaml.
const scheduledTaskOpenAPIV3Schema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "$defs": {
    "ResourceQuantity": {
      "type": "object",
      "properties": {
        "cpu": {
          "type": "string",
          "default": "100m"
        },
        "memory": {
          "type": "string",
          "default": "128Mi"
        }
      }
    },
    "ResourceRequirements": {
      "type": "object",
      "properties": {
        "requests": {
          "$ref": "#/$defs/ResourceQuantity",
          "default": {}
        },
        "limits": {
          "$ref": "#/$defs/ResourceQuantity",
          "default": {}
        }
      }
    }
  },
  "properties": {
    "parameters": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "successfulJobsHistoryLimit": {
          "type": "integer",
          "default": 3,
          "minimum": 0
        },
        "failedJobsHistoryLimit": {
          "type": "integer",
          "default": 1,
          "minimum": 0
        },
        "concurrencyPolicy": {
          "type": "string",
          "default": "Forbid",
          "enum": ["Allow", "Forbid", "Replace"]
        },
        "backoffLimit": {
          "type": "integer",
          "default": 3,
          "minimum": 0,
          "maximum": 10
        },
        "activeDeadlineSeconds": {
          "type": "integer",
          "default": 300,
          "minimum": 1
        },
        "restartPolicy": {
          "type": "string",
          "default": "OnFailure",
          "enum": ["Always", "OnFailure", "Never"]
        }
      }
    },
    "envOverrides": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "schedule": {
          "type": "string",
          "default": "0 0 31 2 *"
        },
        "resources": {
          "$ref": "#/$defs/ResourceRequirements",
          "default": {}
        },
        "imagePullPolicy": {
          "type": "string",
          "default": "IfNotPresent",
          "enum": ["Always", "IfNotPresent", "Never"]
        }
      }
    }
  }
}`

func main() {
	exitCode := 0
	fmt.Println("=== JSON Schema Validation Test (Whole CR Schema) ===")
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 1: Compile the whole service schema
	// ---------------------------------------------------------------
	fmt.Println("--- Test 1: Compile whole service openAPIV3Schema ---")
	serviceSchema, err := compileSchema("service", serviceOpenAPIV3Schema)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		exitCode = 1
	} else {
		fmt.Println("PASS: Whole schema with $defs/$ref compiled successfully")
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 2: Compile the whole scheduled-task schema
	// ---------------------------------------------------------------
	fmt.Println("--- Test 2: Compile whole scheduled-task openAPIV3Schema ---")
	taskSchema, err := compileSchema("scheduled-task", scheduledTaskOpenAPIV3Schema)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		exitCode = 1
	} else {
		fmt.Println("PASS: Whole schema with parameters + envOverrides + $defs compiled")
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 3: Validate correct full input against service schema
	// ---------------------------------------------------------------
	fmt.Println("--- Test 3: Validate correct full input (service) ---")
	validService := `{
		"parameters": {},
		"envOverrides": {
			"replicas": 3,
			"resources": {
				"requests": { "cpu": "200m", "memory": "512Mi" },
				"limits": { "cpu": "1000m", "memory": "1Gi" }
			},
			"imagePullPolicy": "Always"
		}
	}`
	if err := validate(serviceSchema, validService); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		exitCode = 1
	} else {
		fmt.Println("PASS: Full valid input accepted")
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 4: Validate empty parameters + empty envOverrides
	// ---------------------------------------------------------------
	fmt.Println("--- Test 4: Validate empty sections (all defaults) ---")
	if err := validate(serviceSchema, `{"parameters": {}, "envOverrides": {}}`); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		exitCode = 1
	} else {
		fmt.Println("PASS: Empty sections accepted (all fields have defaults)")
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 5: Validate completely empty object
	// ---------------------------------------------------------------
	fmt.Println("--- Test 5: Validate completely empty object {} ---")
	if err := validate(serviceSchema, `{}`); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		exitCode = 1
	} else {
		fmt.Println("PASS: Empty object accepted")
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 6: Reject invalid type in envOverrides.replicas
	// ---------------------------------------------------------------
	fmt.Println("--- Test 6: Reject wrong type (replicas: string) ---")
	bad1 := `{"envOverrides": {"replicas": "three"}}`
	if err := validate(serviceSchema, bad1); err != nil {
		fmt.Printf("PASS: Rejected: %v\n", err)
	} else {
		fmt.Println("FAIL: Should have rejected string for integer")
		exitCode = 1
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 7: Reject invalid nested type via $ref (cpu should be string)
	// ---------------------------------------------------------------
	fmt.Println("--- Test 7: Reject wrong nested type via $ref (cpu: integer) ---")
	bad2 := `{"envOverrides": {"resources": {"requests": {"cpu": 100}}}}`
	if err := validate(serviceSchema, bad2); err != nil {
		fmt.Printf("PASS: Rejected: %v\n", err)
	} else {
		fmt.Println("FAIL: Should have rejected integer for string via $ref")
		exitCode = 1
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 8: Validate correct scheduled-task input
	// ---------------------------------------------------------------
	fmt.Println("--- Test 8: Validate correct full input (scheduled-task) ---")
	validTask := `{
		"parameters": {
			"successfulJobsHistoryLimit": 5,
			"backoffLimit": 2,
			"restartPolicy": "Never"
		},
		"envOverrides": {
			"schedule": "*/5 * * * *",
			"resources": {
				"requests": { "cpu": "50m", "memory": "64Mi" }
			}
		}
	}`
	if err := validate(taskSchema, validTask); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		exitCode = 1
	} else {
		fmt.Println("PASS: Full valid scheduled-task input accepted")
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 9: Reject invalid enum in parameters.restartPolicy
	// ---------------------------------------------------------------
	fmt.Println("--- Test 9: Reject invalid enum (restartPolicy: Sometimes) ---")
	bad3 := `{"parameters": {"restartPolicy": "Sometimes"}}`
	if err := validate(taskSchema, bad3); err != nil {
		fmt.Printf("PASS: Rejected: %v\n", err)
	} else {
		fmt.Println("FAIL: Should have rejected invalid enum")
		exitCode = 1
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 10: Reject value below minimum in parameters.backoffLimit
	// ---------------------------------------------------------------
	fmt.Println("--- Test 10: Reject below minimum (backoffLimit: -1) ---")
	bad4 := `{"parameters": {"backoffLimit": -1}}`
	if err := validate(taskSchema, bad4); err != nil {
		fmt.Printf("PASS: Rejected: %v\n", err)
	} else {
		fmt.Println("FAIL: Should have rejected below-minimum")
		exitCode = 1
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 11: Reject value above maximum in parameters.backoffLimit
	// ---------------------------------------------------------------
	fmt.Println("--- Test 11: Reject above maximum (backoffLimit: 99) ---")
	bad5 := `{"parameters": {"backoffLimit": 99}}`
	if err := validate(taskSchema, bad5); err != nil {
		fmt.Printf("PASS: Rejected: %v\n", err)
	} else {
		fmt.Println("FAIL: Should have rejected above-maximum")
		exitCode = 1
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 12: Additional/unknown properties allowed (schema evolution)
	// ---------------------------------------------------------------
	fmt.Println("--- Test 12: Additional properties allowed (schema evolution) ---")
	extra := `{"envOverrides": {"replicas": 2, "unknownField": "hello"}}`
	if err := validate(serviceSchema, extra); err != nil {
		fmt.Printf("INFO: Rejected: %v\n", err)
		fmt.Println("NOTE: Set additionalProperties: true to allow unknown fields")
	} else {
		fmt.Println("PASS: Additional properties accepted (default JSON Schema behavior)")
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Test 13: Cross-section $ref — envOverrides.resources uses same $defs as top-level
	// ---------------------------------------------------------------
	fmt.Println("--- Test 13: $ref resolves across sections (shared $defs) ---")
	crossRef := `{
		"envOverrides": {
			"resources": {
				"requests": { "cpu": "500m", "memory": "1Gi" },
				"limits": { "cpu": "2000m", "memory": "4Gi" }
			}
		}
	}`
	if err := validate(serviceSchema, crossRef); err != nil {
		fmt.Printf("FAIL: %v\n", err)
		exitCode = 1
	} else {
		fmt.Println("PASS: $ref from envOverrides correctly resolves to top-level $defs")
	}
	fmt.Println()

	// ---------------------------------------------------------------
	// Debug: Simple nested schema without $defs
	// ---------------------------------------------------------------
	fmt.Println("--- Debug: Simple nested schema (no $defs) ---")
	const simpleNested = `{
		"type": "object",
		"properties": {
			"envOverrides": {
				"type": "object",
				"properties": {
					"replicas": { "type": "integer" }
				}
			}
		}
	}`
	simpleSchema, err := compileSchema("simple", simpleNested)
	if err != nil {
		fmt.Printf("FAIL compile: %v\n", err)
		exitCode = 1
	} else {
		// Should reject
		if err := validate(simpleSchema, `{"envOverrides": {"replicas": "bad"}}`); err != nil {
			fmt.Printf("PASS: Simple nested rejected invalid: %v\n", err)
		} else {
			fmt.Println("FAIL: Simple nested accepted invalid type!")
		}
		// Should accept
		if err := validate(simpleSchema, `{"envOverrides": {"replicas": 5}}`); err != nil {
			fmt.Printf("FAIL: Simple nested rejected valid: %v\n", err)
		} else {
			fmt.Println("PASS: Simple nested accepted valid")
		}
	}
	fmt.Println()

	// Debug: Same structure as service but WITHOUT $defs
	fmt.Println("--- Debug: Nested with NO $defs ---")
	const nestedNoDefs = `{
		"type": "object",
		"properties": {
			"parameters": {
				"type": "object",
				"properties": {}
			},
			"envOverrides": {
				"type": "object",
				"properties": {
					"replicas": { "type": "integer", "default": 1 },
					"imagePullPolicy": { "type": "string", "default": "IfNotPresent" }
				}
			}
		}
	}`
	noDefsSchema, err := compileSchema("noDefs", nestedNoDefs)
	if err != nil {
		fmt.Printf("FAIL compile: %v\n", err)
		exitCode = 1
	} else {
		if err := validate(noDefsSchema, `{"envOverrides": {"replicas": "bad"}}`); err != nil {
			fmt.Printf("PASS: Without $defs, rejected invalid: %v\n", err)
		} else {
			fmt.Println("FAIL: Without $defs, accepted invalid type!")
		}
	}
	fmt.Println()

	// Debug: Same structure WITH $defs but $ref NOT used
	fmt.Println("--- Debug: Has $defs but $ref NOT used ---")
	const hasDefsNotUsed = `{
		"type": "object",
		"$defs": {
			"Foo": { "type": "string" }
		},
		"properties": {
			"envOverrides": {
				"type": "object",
				"properties": {
					"replicas": { "type": "integer" }
				}
			}
		}
	}`
	defsNotUsedSchema, err := compileSchema("defsNotUsed", hasDefsNotUsed)
	if err != nil {
		fmt.Printf("FAIL compile: %v\n", err)
		exitCode = 1
	} else {
		if err := validate(defsNotUsedSchema, `{"envOverrides": {"replicas": "bad"}}`); err != nil {
			fmt.Printf("PASS: With unused $defs, rejected invalid: %v\n", err)
		} else {
			fmt.Println("FAIL: With unused $defs, accepted invalid type!")
		}
	}
	fmt.Println()

	// Debug: Has $defs AND $ref IS used + other properties
	fmt.Println("--- Debug: Has $defs with $ref used alongside plain properties ---")
	const hasDefsWithRef = `{
		"type": "object",
		"$defs": {
			"Foo": { "type": "object", "properties": { "x": { "type": "string" } } }
		},
		"properties": {
			"envOverrides": {
				"type": "object",
				"properties": {
					"replicas": { "type": "integer" },
					"bar": { "$ref": "#/$defs/Foo" }
				}
			}
		}
	}`
	defsWithRefSchema, err := compileSchema("defsWithRef", hasDefsWithRef)
	if err != nil {
		fmt.Printf("FAIL compile: %v\n", err)
		exitCode = 1
	} else {
		if err := validate(defsWithRefSchema, `{"envOverrides": {"replicas": "bad"}}`); err != nil {
			fmt.Printf("PASS: With $ref used, rejected invalid: %v\n", err)
		} else {
			fmt.Println("FAIL: With $ref used, accepted invalid type!")
		}
	}
	fmt.Println()

	// Debug: $ref with default sibling (the actual pattern from service schema)
	fmt.Println("--- Debug: $ref with default sibling ---")
	const refWithDefault = `{
		"type": "object",
		"$defs": {
			"Foo": { "type": "object", "properties": { "x": { "type": "string" } } }
		},
		"properties": {
			"envOverrides": {
				"type": "object",
				"properties": {
					"replicas": { "type": "integer" },
					"bar": { "$ref": "#/$defs/Foo", "default": {} }
				}
			}
		}
	}`
	refDefaultSchema, err := compileSchema("refDefault", refWithDefault)
	if err != nil {
		fmt.Printf("FAIL compile: %v\n", err)
		exitCode = 1
	} else {
		if err := validate(refDefaultSchema, `{"envOverrides": {"replicas": "bad"}}`); err != nil {
			fmt.Printf("PASS: $ref+default, rejected invalid: %v\n", err)
		} else {
			fmt.Println("FAIL: $ref+default, accepted invalid type!")
		}
	}
	fmt.Println()

	// Debug: Flat schema (same as earlier working test)
	fmt.Println("--- Debug: Flat schema (no nesting) ---")
	const flat = `{
		"type": "object",
		"properties": {
			"replicas": { "type": "integer" }
		}
	}`
	flatSchema, err := compileSchema("flat", flat)
	if err != nil {
		fmt.Printf("FAIL compile: %v\n", err)
		exitCode = 1
	} else {
		if err := validate(flatSchema, `{"replicas": "bad"}`); err != nil {
			fmt.Printf("PASS: Flat rejected invalid: %v\n", err)
		} else {
			fmt.Println("FAIL: Flat accepted invalid type!")
		}
	}
	fmt.Println()

	// ---------------------------------------------------------------
	fmt.Println("=== Summary ===")
	if exitCode == 0 {
		fmt.Println("All tests passed!")
		fmt.Println("The whole openAPIV3Schema (with $defs, parameters, envOverrides as siblings)")
		fmt.Println("is valid JSON Schema and works with standard parsers.")
	} else {
		fmt.Println("Some tests failed.")
	}
	os.Exit(exitCode)
}

func compileSchema(name, schemaJSON string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	uri := "https://openchoreo.dev/schemas/" + name + ".json"
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaJSON))
	if err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	if err := c.AddResource(uri, doc); err != nil {
		return nil, fmt.Errorf("add resource: %w", err)
	}
	return c.Compile(uri)
}

func validate(schema *jsonschema.Schema, dataJSON string) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	return schema.Validate(unmarshalToAny(dataJSON))
}

func unmarshalToAny(jsonStr string) any {
	var v any
	if err := json.Unmarshal([]byte(jsonStr), &v); err != nil {
		panic(fmt.Sprintf("invalid JSON: %v", err))
	}
	return v
}
