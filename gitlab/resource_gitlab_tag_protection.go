package gitlab

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	gitlab "github.com/xanzy/go-gitlab"
)

func resourceGitlabTagProtection() *schema.Resource {
	acceptedAccessLevels := make([]string, 0, len(tagProtectionAccessLevelID))

	for k := range tagProtectionAccessLevelID {
		acceptedAccessLevels = append(acceptedAccessLevels, k)
	}
	return &schema.Resource{
		Description: "This resource allows you to protect a specific tag or wildcard by an access level so that the user with less access level cannot Create the tags.",

		CreateContext: resourceGitlabTagProtectionCreate,
		ReadContext:   resourceGitlabTagProtectionRead,
		DeleteContext: resourceGitlabTagProtectionDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"project": {
				Description: "The id of the project.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"tag": {
				Description: "Name of the tag or wildcard.",
				Type:        schema.TypeString,
				ForceNew:    true,
				Required:    true,
			},
			"create_access_level": {
				Type:             schema.TypeString,
				ValidateDiagFunc: validateValueFunc(acceptedAccessLevels),
				Required:         true,
				ForceNew:         true,
			},
		},
	}
}

func resourceGitlabTagProtectionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	project := d.Get("project").(string)
	tag := gitlab.String(d.Get("tag").(string))
	createAccessLevel := tagProtectionAccessLevelID[d.Get("create_access_level").(string)]

	options := &gitlab.ProtectRepositoryTagsOptions{
		Name:              tag,
		CreateAccessLevel: &createAccessLevel,
	}

	log.Printf("[DEBUG] create gitlab tag protection on %v for project %s", options.Name, project)

	tp, _, err := client.ProtectedTags.ProtectRepositoryTags(project, options, gitlab.WithContext(ctx))
	if err != nil {
		// Remove existing tag protection
		_, err = client.ProtectedTags.UnprotectRepositoryTags(project, *tag, gitlab.WithContext(ctx))
		if err != nil {
			return diag.FromErr(err)
		}
		// Reprotect tag with updated values
		tp, _, err = client.ProtectedTags.ProtectRepositoryTags(project, options, gitlab.WithContext(ctx))
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(buildTwoPartID(&project, &tp.Name))

	return resourceGitlabTagProtectionRead(ctx, d, meta)
}

func resourceGitlabTagProtectionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	project, tag, err := projectAndTagFromID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[DEBUG] read gitlab tag protection for project %s, tag %s", project, tag)

	pt, _, err := client.ProtectedTags.GetProtectedTag(project, tag, gitlab.WithContext(ctx))
	if err != nil {
		if is404(err) {
			log.Printf("[DEBUG] gitlab tag protection not found %s/%s", project, tag)
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	accessLevel, ok := tagProtectionAccessLevelNames[pt.CreateAccessLevels[0].AccessLevel]
	if !ok {
		return diag.Errorf("tag protection access level %d is not supported. Supported are: %v", pt.CreateAccessLevels[0].AccessLevel, tagProtectionAccessLevelNames)
	}

	accessLevel, ok := tagProtectionAccessLevelNames[pt.CreateAccessLevels[0].AccessLevel]
	if !ok {
		return fmt.Errorf("tag protection access level %d is not supported. Supported are: %v", pt.CreateAccessLevels[0].AccessLevel, tagProtectionAccessLevelNames)
	}

	d.Set("project", project)
	d.Set("tag", pt.Name)
	d.Set("create_access_level", accessLevel)

	d.SetId(buildTwoPartID(&project, &pt.Name))

	return nil
}

func resourceGitlabTagProtectionDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*gitlab.Client)
	project := d.Get("project").(string)
	tag := d.Get("tag").(string)

	log.Printf("[DEBUG] Delete gitlab protected tag %s for project %s", tag, project)

	_, err := client.ProtectedTags.UnprotectRepositoryTags(project, tag, gitlab.WithContext(ctx))
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func projectAndTagFromID(id string) (string, string, error) {
	project, tag, err := parseTwoPartID(id)

	if err != nil {
		log.Printf("[WARN] cannot get group member id from input: %v", id)
	}
	return project, tag, err
}
