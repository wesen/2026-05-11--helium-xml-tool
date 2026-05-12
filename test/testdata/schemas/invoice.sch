<?xml version="1.0"?>
<schema xmlns="http://www.ascc.net/xml/schematron">
  <pattern name="invoice-check">
    <rule context="invoice">
      <assert test="total">Invoice must have a total</assert>
    </rule>
    <rule context="line">
      <assert test="@quantity > 0">Line quantity must be positive</assert>
    </rule>
  </pattern>
</schema>
