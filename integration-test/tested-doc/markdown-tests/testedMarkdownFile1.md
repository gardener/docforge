---
title: testedFile1
---
# Tested markdown file 1

### Link file which is in the structure
[test1](https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/markdown-tests/testedMarkdownFile2.md)

### Link file which is not in the structure
[test2](https://github.com/gardener/gardener/blob/v1.30.0/README.md)

### Link relatively file which is in the structure
[test3](testedDir/testedMarkdownFile3.md)

### Link relatively another file which is in the structure
[test4](testedDir/innerDir/testedMarkdownFile5.md)

### Link existing image with absolute path
![test5](https://github.com/gardener/docforge/blob/master/integration-test/tested-doc/images/photo1.jpeg)

### Link existing image with relative path
![test6](../images/photo2.jpeg)

### Link existing image with relative path and title
![test7](./../images/photo3.jpeg "Photo3")
