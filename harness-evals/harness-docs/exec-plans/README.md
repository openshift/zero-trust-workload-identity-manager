# Execution Plans

> - What are exec-plans?
> - When to create them
> - How to use them
> - Completion workflow
> - Template

## Component-Specific Exec-Plans

Active exec-plans for ZTWIM are tracked in `active/`:

```text
exec-plans/
└── active/              # Create feature-specific exec-plans here
```

## Usage

```bash
# Get template from Platform
curl -O https://raw.githubusercontent.com/openshift/enhancements/master/ai-docs/workflows/exec-plans/template.md

# Create exec-plan
mv template.md active/feature-name.md

# Fill in and track during implementation
```

