# Using `Alias`
Aliases "virtually move" content to another place. A page can have multiple aliases. If a dir has an alias it's like the whole directory is being "virtually moved".

Manifest: dir.yaml
```yaml
structure:
- dir: section
  frontmatter:
    aliases:
    # all children will have alias /dirmove/blogs/<path_from_here_to_child>
    - /dirmove/blogs
  structure:
  - file: https://github.com/gardener/docforge/blob/master/docs/manifests.md
    frontmatter:
      title: Guides
      description: Walkthroughs of common activities
      aliases:
      - /root/file/
- file: overview
  source: https://github.com/gardener/docforge/blob/master/docs/README.md
```
Result: section/apiserver.md
``` yaml
---
title: Guides
description: Walkthroughs of common activities
aliases:
- /root/file/
- /dirmove/blogs/apiserver/
---
```