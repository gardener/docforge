---
title: testedFile1
---

# Tested markdown file 1

### Link file which is not in the structure
[test1](https://github.com/gardener/gardener/blob/v1.30.0/README.md)

### Link relatively file which is in the structure
[test2](/maintree/markdown-tests/filetree/testedmarkdownfile3/)

### Link relatively another file which is in the structure
[test3](/maintree/markdown-tests/filetree/innerdir/testedmarkdownfile5/)

### Link existing image with relative path
![test4](/maintree/gardener-docforge-logo.png)

### Link existing image with relative path and title
![test5](/maintree/gardener-docforge-logo.png "gardener-docforge-logo")

### Link outside image
![test6](https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png)
