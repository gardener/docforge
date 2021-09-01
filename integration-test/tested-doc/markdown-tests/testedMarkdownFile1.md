---
title: testedFile1
---
# Tested markdown file 1

### Link file which is not in the structure
[test1](https://github.com/gardener/gardener/blob/v1.30.0/README.md)

### Link relatively file which is in the structure
[test2](testedDir/testedMarkdownFile3.md)

### Link relatively another file which is in the structure
[test3](testedDir/innerDir/testedMarkdownFile5.md)

### Link existing image with relative path
![test4](../images/gardener-docforge-logo.png)

### Link existing image with relative path and title
![test5](./../images/gardener-docforge-logo.png "gardener-docforge-logo")
