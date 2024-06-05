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
![test4](/baseURL/__resources/gardener-docforge-logo_051125.png)

### Link existing image with relative path and title
![test5](/baseURL/__resources/gardener-docforge-logo_051125.png "gardener-docforge-logo")

### Link outside image
![test6](https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png)
