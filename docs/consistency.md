# Preserving Pulled Documents Consistency
Pulled Markdown documents are very likely to contain links to other resources, such as multimedia files (e.g. images), locations in the same document (e.g. a section head), other Markdown documents, including links ot other downloaded material, or any websites. Simply moving material may break the documents references to such resources, particularly when the links are relative. In addition, some referenced resources may need to be downloaded. Considering they might be referenced also from multiple documents that may reside in completely different relative locations after their pull, such resources and links to them need special attention too. An important aspect of working with documents with docforge is therefore maintaining links consistency.

## Links in Markdown Documents
The links that will be processed are anything that falls in this scope:
- All forms of image, hyperlink or autolink markdown as specified in [Commonmark](https://spec.commonmark.org) and the [GitHub](https://github.github.com/gfm) flavored markdown.
- Any HTML element with "src" or "href" attribute, because Markdown permits raw HTML, and it's fairly common practice to make use of that.

## Links to documents
Markdown documents are downloaded only if they are document nodes in the documentation structure. All cross-links to downloaded documents are *converted to relative*. The links destinations are calculated and adjusted to reflect correctly the potentially new location of the referenced documents, defined in the documentation structure. This applies both to originally relative and absolute links and links between GitHub repositories.

If a linked document is not a document node in the documentation model, then it will not be downloaded and the link to it is rewritten to its resolved absolute form.

Cascading download of documents based on hyperlinks in their content is not supported intentionally to ensure predictable results and avoid accidental downloads.

## Links to resources

Embedded resources (images and other non-document files) referenced by downloaded Markdown documents are **not downloaded** by docforge unless they appear as explicit nodes in the manifest. When a `.md` file references an image via a relative link, docforge rewrites the link to its absolute raw GitHub URL so the reference remains valid in the output bundle. No resource is renamed or copied to the destination.

If you need a resource to be present locally in the output bundle, declare it explicitly in the manifest as a `file` node alongside the documents that reference it.

Absolute links that do not need to be processed because of a reason outlined so far are left intact.

## Links to internal document sections
Internal document links (e.g. `#heading-section-id`) are not processed and are left as is.

## Other links
Links with `mailto:` protocol scheme are not processed.
Any other absolute links are not processed.  
Any other relative links are converted to absolute.  
