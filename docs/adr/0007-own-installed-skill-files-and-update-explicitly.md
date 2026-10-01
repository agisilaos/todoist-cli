# Own installed skill files and update explicitly

Bundle the Todoist agent skill with the CLI and install it only for an explicitly selected target, scope, and absolute destination. Record owned files and their content hashes; update explicitly from the running binary rather than automatically downloading or rewriting detected installations, trading convenience for alignment with available commands and predictable control over user customization.

An existing filename is not evidence of ownership. Modified managed files block replacement and removal by default; explicit update can preserve originals in a separate backup before replacement, and explicit uninstall can leave modified files behind. Unrelated content is always retained. These boundaries deliberately avoid adopting arbitrary existing skills or editing shared agent instructions, and leave agent discovery and consumption to separately verified target behavior.
