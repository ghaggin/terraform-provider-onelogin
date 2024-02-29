package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/ghaggin/terraform-provider-onelogin/onelogin"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type Generator struct {
	client      *onelogin.Client
	nameTracker map[string]bool
}

func NewGenerator(client *onelogin.Client) *Generator {
	return &Generator{
		client:      client,
		nameTracker: map[string]bool{},
	}
}

func (g *Generator) Run(outputDir string, resource []string) error {
	finfo, err := os.Stat(outputDir)
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(outputDir, 0755)
	} else if err == nil && !finfo.IsDir() {
		return fmt.Errorf("output directory name exists as file: %s", outputDir)
	}

	if err != nil {
		return err
	}

	for _, r := range resource {
		// reset the name tracker for each resource
		g.nameTracker = map[string]bool{}

		// create file for resource
		filePath := fmt.Sprintf("%s/%s.tf", outputDir, r)
		f, err := os.Create(filePath)
		if err != nil {
			return err
		}
		defer f.Close()

		// create hcl file object
		hclf := hclwrite.NewFile()
		rootBody := hclf.Body()

		switch r {
		case "roles":
			err := g.runRoles(rootBody)
			if err != nil {
				return err
			}
		case "mappings":
			err := g.runMappings(rootBody)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid resource specified: %s", r)
		}

		// write hcl file to disk
		_, err = f.Write(hclf.Bytes())
		if err != nil {
			return err
		}
		f.Close()
	}
	return nil
}

func (g *Generator) runRoles(rootBody *hclwrite.Body) error {
	numRoles := 0

	for page := 1; ; page++ {
	retry:
		var resp []onelogin.Role
		err := g.client.ExecRequestPaged(&onelogin.Request{
			Method:    onelogin.MethodGet,
			Path:      onelogin.PathRoles,
			RespModel: &resp,
		}, &onelogin.Page{
			Limit: 50,
			Page:  page,
		})
		if err != nil && err == onelogin.ErrBadGateway {
			fmt.Fprintln(os.Stderr, "retrying")
			goto retry
		}

		if err != nil && err != onelogin.ErrNoMorePages {
			return err
		}

		fmt.Fprintf(os.Stderr, "got role page: %v size: %v\n", page, len(resp))
		numRoles += len(resp)

		g.processRoles(rootBody, resp)

		if err != nil && err == onelogin.ErrNoMorePages {
			break
		}
	}

	fmt.Fprintf(os.Stderr, "got %v roles\n", numRoles)

	return nil
}

func (g *Generator) processRoles(rootBody *hclwrite.Body, roles []onelogin.Role) error {
	for _, r := range roles {
		resourceName := g.normalizeName(r.Name, nil)

		importBlock := rootBody.AppendNewBlock("import", []string{})
		importBlock.Body().SetAttributeValue("id", cty.NumberIntVal(r.ID))
		importBlock.Body().SetAttributeTraversal("to", hcl.Traversal{
			hcl.TraverseRoot{Name: "onelogin_role"},
			hcl.TraverseAttr{Name: resourceName},
		})

		rootBody.AppendNewline()

		resourceBlock := rootBody.AppendNewBlock("resource", []string{"onelogin_role", resourceName})
		resourceBlock.Body().SetAttributeValue("name", cty.StringVal(r.Name))
		if len(r.Apps) > 0 {
			apps := []cty.Value{}
			for _, a := range r.Apps {
				apps = append(apps, cty.NumberIntVal(a))
			}
			resourceBlock.Body().SetAttributeValue("apps", cty.ListVal(apps))
		}

		if len(r.Admins) > 0 {
			admins := []cty.Value{}
			for _, a := range r.Admins {
				admins = append(admins, cty.NumberIntVal(a))
			}
			resourceBlock.Body().SetAttributeValue("admins", cty.ListVal(admins))
		}

		rootBody.AppendNewline()
	}

	return nil
}

func (g *Generator) runMappings(rootBody *hclwrite.Body) error {
	var respEnabled []onelogin.Mapping
	err := g.client.ExecRequest(&onelogin.Request{
		Method:    onelogin.MethodGet,
		Path:      onelogin.PathMappings,
		RespModel: &respEnabled,
	})
	if err != nil {
		return err
	}

	for _, m := range respEnabled {
		if m.Position == nil {
			panic(fmt.Sprintf("mapping %s has no position", m.Name))
		}
	}

	sort.Slice(respEnabled, func(i, j int) bool {
		return *respEnabled[i].Position < *respEnabled[j].Position
	})

	var respDisabled []onelogin.Mapping
	err = g.client.ExecRequest(&onelogin.Request{
		Method:    onelogin.MethodGet,
		Path:      onelogin.PathMappings,
		RespModel: &respDisabled,
		QueryParams: onelogin.QueryParams{
			"enabled": "false",
		},
	})
	if err != nil {
		return err
	}

	mappings := []onelogin.Mapping{}
	mappings = append(mappings, respEnabled...)
	mappings = append(mappings, respDisabled...)

	mappingOrderEnabled := []string{}
	mappingOrderDisabled := []string{}

	for _, m := range mappings {
		resourceName := g.normalizeName(m.Name, []replacement{
			{`16`, "sixteen"},
		})

		// Add to the config order
		if m.Enabled {
			mappingOrderEnabled = append(mappingOrderEnabled, resourceName)
		} else {
			mappingOrderDisabled = append(mappingOrderDisabled, resourceName)
		}

		// Create mapping config and immport config

		importBlock := rootBody.AppendNewBlock("import", []string{})
		importBlock.Body().SetAttributeValue("id", cty.NumberIntVal(m.ID))
		importBlock.Body().SetAttributeTraversal("to", hcl.Traversal{
			hcl.TraverseRoot{Name: "onelogin_mapping"},
			hcl.TraverseAttr{Name: resourceName},
		})

		rootBody.AppendNewline()

		resourceBlock := rootBody.AppendNewBlock("resource", []string{"onelogin_mapping", resourceName})
		resourceBlock.Body().SetAttributeValue("name", cty.StringVal(m.Name))
		resourceBlock.Body().SetAttributeValue("match", cty.StringVal(m.Match))

		conditions := []cty.Value{}
		for _, c := range m.Conditions {
			conditions = append(conditions, cty.ObjectVal(map[string]cty.Value{
				"source":   cty.StringVal(c.Source),
				"operator": cty.StringVal(c.Operator),
				"value":    cty.StringVal(c.Value),
			}))
		}
		resourceBlock.Body().SetAttributeValue("conditions", cty.ListVal(conditions))

		actions := []cty.Value{}
		for _, a := range m.Actions {
			values := []cty.Value{}
			for _, v := range a.Value {
				values = append(values, cty.StringVal(v))
			}

			actions = append(actions, cty.ObjectVal(map[string]cty.Value{
				"action": cty.StringVal(a.Action),
				"value":  cty.ListVal(values),
			}))
		}

		resourceBlock.Body().SetAttributeValue("actions", cty.ListVal(actions))

		rootBody.AppendNewline()
	}

	// Add the config order
	resourceBlock := rootBody.AppendNewBlock("resource", []string{"onelogin_mapping_order", "instance"})
	resourceBody := resourceBlock.Body()

	enabledTraversal := hclwrite.Tokens{}
	enabledTraversal = append(enabledTraversal, &hclwrite.Token{
		Type:  hclsyntax.TokenOBrack,
		Bytes: []byte("[\n"),
	})
	for _, m := range mappingOrderEnabled {
		t := &hclwrite.Token{
			Type:  hclsyntax.TokenStringLit,
			Bytes: []byte("    onelogin_mapping." + m + ".id,\n"),
		}

		enabledTraversal = append(enabledTraversal, t)
	}
	enabledTraversal[len(enabledTraversal)-1] = &hclwrite.Token{
		Type:  hclsyntax.TokenCBrack,
		Bytes: []byte("  ]"),
	}

	disabledTraversal := hclwrite.Tokens{}
	disabledTraversal = append(disabledTraversal, &hclwrite.Token{
		Type:  hclsyntax.TokenStringLit,
		Bytes: []byte("[\n"),
	})
	for _, m := range mappingOrderDisabled {
		t := &hclwrite.Token{
			Type:  hclsyntax.TokenStringLit,
			Bytes: []byte("    onelogin_mapping." + m + ".id,\n"),
		}

		disabledTraversal = append(disabledTraversal, t)
	}
	disabledTraversal[len(disabledTraversal)-1] = &hclwrite.Token{
		Type:  hclsyntax.TokenCBrack,
		Bytes: []byte("  ]"),
	}

	resourceBody.SetAttributeRaw("enabled", enabledTraversal)
	resourceBody.SetAttributeRaw("disabled", disabledTraversal)

	return nil
}

type replacement struct {
	regex string
	with  string
}

func (g *Generator) normalizeName(s string, replacements []replacement) string {
	s = strings.ToLower(s)

	// append the standard replacements to inputted ones
	if replacements == nil {
		replacements = []replacement{}
	}
	replacements = append(replacements, []replacement{
		{` |-|\(|\)|:|\.|,|/|{|}|#`, "_"},
		{`_$|^_`, ""},
		{`_+`, "_"},
		{`&`, "and"},
		{`@`, "at"},
		{`'`, ""},
		{`"`, ""},
	}...)

	for _, r := range replacements {
		s = regexp.MustCompile(r.regex).ReplaceAllString(s, r.with)
	}

	s = regexp.MustCompile(`^(\d)`).ReplaceAllString(s, "r_$1")

	baseName := s
	for i := 1; ; i++ {
		if _, ok := g.nameTracker[s]; !ok {
			g.nameTracker[s] = true
			break
		}

		s = fmt.Sprintf("%s_%d", baseName, i)
	}

	return s
}
