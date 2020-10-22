# Preserving Pulled Documents Consistency
Pulled markdown documents are very likely to contain links to other resources, such as multimedia files (e.g. images), locations in the same document (e.g. a section head), other markdown documents, including links ot other downloaded material, or any websites. Simply moving material may break the documents references to such resources, particularly when the links are relative. In addition, some referenced resources may need to be downloaded. Considering they might be referenced also from multiple documents that may reside in completely different relative locations after their pull, such resources and links to them need special attention too. An important aspect of working with documents with docforge is therefore maintaining links consistency.

## Links in Markdown Documents
The links that will be processed are anything that falls in this scope:
- All forms of image, hyperlink or autolink markdown as specified in [Commonmark](https://spec.commonmark.org) and the [GitHub](https://github.github.com/gfm) flavored markdown.
- Any HTML element with "src" or "href" attribute, because Markdown permits raw HTML and it's fairly common practice to make use of that.

## Links to documents
Markdown documents are downloaded only if they are document nodes in the documentation structure. All cross links to downloaded documents are *converted to relative*. The links destinations are calculated and adjusted to reflect correctly the potentially new location of the referenced documents, defined in the documentation structure. This applies both to originally relative and absolute links and links between GitHub repositories.

If a linked document is not a document node in the documentation model, then it will not be downloaded and the link to it is rewritten to its resolved absolute form.

Cascading download of documents based on hyperlinks in their content is not supported intentionally to ensure predictable results and avoid accidental downloads.

## Links to resources
Resources linked by downloaded documents are downloaded if they are in a linking document's *download scope*.

The definition of a "download scope" is the information space where all resources can be considered local to each other adn. The actual interpretation of such definition will vary across source systems and project organizations. For documents and resources stored in GitHub, the natural download scope are the repositories where they are managed. A download scope may also be interpreted by a logical criteria, not only physical. In a project, the download scope may be a component, that is spread across several GitHub repositories.

Resources are downloaded in a dedicated destination, with their names changed by default to UUIDs to avoid potential name clashes. Links in all downloaded documents originally referencing a resource that has been downloaded and processed like that are adjusted according to the documents relative position to the new location of the resource and rewritten as *relative* links. The new name of the resource is used in the document links referencing it. A resources is downloaded only once, regardless of how many documents reference it.

Link adjustment to downloaded resource and its rewrite to relative form applies to all downloaded documents that reference that resource in any way regardless of their download scope. 

Absolute links that do not need to be processed because of a reason outlined so far are left in tact.
Relative links that have not been processed for a reason outlined so far are converted to absolute.

## Links to internal document sections
Internal document links (e.g. `#heading-section-id`) are not processed and are left as is, unless the document section they reference has been removed by an exclusion pattern. In this case, the link is removed.

## Other links
Internal document fragment references (#) are not processed.
Links with `mailto:` protocol scheme are not processed.

Any other absolute links are not processed. Any other relative links are converted to absolute.
Examples:
- https://www.sap.com
- https://github.com/gardener/gardener/tree/master/example
- ../charts

## Linked documents and resources versions
The topic is discussed in detail in the [versions](versions.md) topic.
