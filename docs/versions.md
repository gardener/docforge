# Versions

## Reproducible documentation bundles

What we want are **reproducible documentation bundles**, which can even be a legal obligation for companies delivering software. There are different strategies how to ensure that:
- Reconstruct documentation bundle
This method builds on a reliable mechanism for reconstructing documentation bundle from content source repositories history. It doesn't need to store the built bundles as longs as it can rebuild them at any point. The only thing that is of interest is the documentation bundle configuration, which is most likely versioned and a tool that can reconstruct the bundle with it at any point in time.
A key assumption in this method is that the source content repositories are reliably controlled so that their history is immutable and  neither commits that may be referenced by a documentation bundle, nor whole repositories can disappear .
Obviously, the topic for versions is essential  in this context.
- Backup and restore documentation bundle
This method utilizes a dedicated controlled repository to reliably backup the history of documentation bundles versions. A particular version can be restored upon request then. It is a safe bet that doubles the amount of manage information, but is also a valid approach. Versions are not so important in this case as builds are most likely created against latest version (master).

In the next sections we will discuss versioned links handling in the context of the first method - reconstructing documentation bundles. 

## Cross links to other documents that are part of the documentation model

The versions of such documents are encoded in the documents source content links in the structure, and once downloaded links to them are converted to relative. However chances are high that the document is linked with another version. For example, it is a common practice that links to resources are created against the head of the *master* branch in a GitHub repo where they reside. That can present a certain problem - the link does not point to a stable variant of a resource, but to the head tip in its history. Therefore it is not possible to reconstruct a particular documentation state when it includes such *dynamic* links.

Example:  Consider a documentation structure with two nodes with content sources, where the content for `Node 1` has a link referencing content from `Node 2`:
```
- name: Node 1
  source: https://github.com/gardener/gardener-extension-provider-aws/blob/master/docs/usage-as-end-user.md 
- name: Node 2
  source: - https://github.com/gardener/gardener/blob/v10.0.0/docs/usage/shoot_operations.md
```
Consider that the reference link between the two is `https://github.com/gardener/gardener/blob/master/docs/usage/shoot_operations.md `. Note the difference: `v10.0.0` (concrete version) vs `master` (tip of history - changing with each commit in https://github.com/gardener/gardener)

It is important to recognize that both links are for different versions of the same resource and because the referenced resource is a documentation node and will be downloaded anyway no download action needs to be taken. Then there are two cases to consider: 
- If the link is autolink and needs to be preserved in this form it has to be rewritten to match the document node version, i.e. to `https://github.com/gardener/gardener/blob/v10.0.0/docs/usage/shoot_operations.md` 
- In all other cases it is only necessary to rewrite the link to relative, i.e. `./shoot_operations.md`

## Links to resource in the download scope

Resources in the download scope of the documentation are downloaded, but it is important to realize, which version to consider for download, because similar to the case above it is common that they are linked using `master`.
It is convenient for the download scope to have a concept for version. Then all resources within a particular download scope will be considered with that version. If they are specified with another, such as `master`, it will be dismissed and replaced with the download scope version. 

For example, if the download scope for a set of documents is the repository X in its version x.y.z, all resources from it referenced in (other) documents are downloaded with their version X.

## Links to documents and resources not in the download scope

Non download scope links are considered "external" and beyond control. The safest bet is to not try to make changes to them.

One exception are the relative links that have been converted to absolute because their destinations are not part of the download scope. For those links we can still establish their relevant version and -consider it in the absolute link. The relevant link version will be the same as the one for the download scope to which they were relative.

## Resolving multiple versions

It is possible that several documents reference several different versions of a resource/document. Different strategies can be implemented to handle this:
- Report an error and expect that the inconsistency (if unintended) is fixed in the content
- Consider multiple referenced resources versions. All non-master versions are downloaded and document references to a version a preserved but with relative links to downloaded content. Master version of the resource is processed as described above.
- Ignore different versions and process references according to the rules above for 'master' links 