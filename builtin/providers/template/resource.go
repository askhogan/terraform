package template

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"

	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/config/lang"
	"github.com/hashicorp/terraform/config/lang/ast"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/mitchellh/go-homedir"
)

func resource() *schema.Resource {
	return &schema.Resource{
		Create: Create,
		Read:   Read,
		Delete: Delete,

		Schema: map[string]*schema.Schema{
			"filename": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "file to read template from",
				ForceNew:    true,
			},
			"vars": &schema.Schema{
				Type:        schema.TypeMap,
				Optional:    true,
				Default:     make(map[string]interface{}),
				Description: "variables to substitute",
				ForceNew:    true,
			},
			"rendered": &schema.Schema{
				Type:        schema.TypeString,
				Computed:    true,
				Description: "rendered template",
			},
		},
	}
}

func Create(d *schema.ResourceData, meta interface{}) error {
	return eval(d)
}
func Delete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}

var readfile func(string) ([]byte, error) = ioutil.ReadFile // testing hook

func Read(d *schema.ResourceData, meta interface{}) error {
	// Catch rendering errors here.
	rendered, err := render(d)
	if err != nil {
		return err
	}
	if hash(rendered) != d.Id() {
		d.Set("rendered", rendered)
	}
	return nil
}

func eval(d *schema.ResourceData) error {
	rendered, err := render(d)
	if err != nil {
		return err
	}
	d.Set("rendered", rendered)
	d.SetId(hash(rendered))
	return nil
}

func render(d *schema.ResourceData) (string, error) {
	filename := d.Get("filename").(string)
	vars := d.Get("vars").(map[string]interface{})

	path, err := homedir.Expand(filename)
	if err != nil {
		return "", err
	}

	buf, err := readfile(path)
	if err != nil {
		return "", err
	}

	rendered, err := execute(string(buf), vars)
	if err != nil {
		return "", fmt.Errorf("failed to render %v: %v", filename, err)
	}

	return rendered, nil
}

// execute parses and executes a template using vars.
func execute(s string, vars map[string]interface{}) (string, error) {
	root, err := lang.Parse(s)
	if err != nil {
		return "", err
	}

	varmap := make(map[string]ast.Variable)
	for k, v := range vars {
		// As far as I can tell, v is always a string.
		// If it's not, tell the user gracefully.
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("unexpected type for variable %q: %T", k, v)
		}
		varmap[k] = ast.Variable{
			Value: s,
			Type:  ast.TypeString,
		}
	}

	cfg := lang.EvalConfig{
		GlobalScope: &ast.BasicScope{
			VarMap:  varmap,
			FuncMap: config.Funcs,
		},
	}

	out, typ, err := lang.Eval(root, &cfg)
	if err != nil {
		return "", err
	}
	if typ != ast.TypeString {
		return "", fmt.Errorf("unexpected output ast.Type: %v", typ)
	}

	return out.(string), nil
}

func hash(s string) string {
	sha := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sha[:])[:20]
}
