<?xml version="1.0"?>
<!-- Schematron schema for book validation -->
<schema xmlns="http://www.ascc.net/xml/schematron">
  <pattern name="book-rules">
    <rule context="book">
      <assert test="title">Book must have a title</assert>
      <assert test="author or editor">Book must have an author or editor</assert>
    </rule>
    <rule context="book/title">
      <assert test="string-length(.) > 0">Title must not be empty</assert>
    </rule>
  </pattern>
</schema>
