<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
                xmlns:f="http://example.com/functions"
                version="3.0">

  <xsl:output method="xml" indent="yes"/>

  <!-- Global parameter -->
  <xsl:param name="debug" select="false()"/>

  <!-- Global variable -->
  <xsl:variable name="version" select="'1.0'"/>

  <!-- Main template -->
  <xsl:template match="/">
    <output>
      <xsl:apply-templates select="*"/>
    </output>
  </xsl:template>

  <!-- Book template -->
  <xsl:template match="book">
    <book-entry>
      <xsl:apply-templates select="title"/>
      <xsl:apply-templates select="author"/>
    </book-entry>
  </xsl:template>

  <!-- Title template -->
  <xsl:template match="title">
    <title-text><xsl:value-of select="."/></title-text>
  </xsl:template>

  <!-- Author template -->
  <xsl:template match="author">
    <author-name><xsl:value-of select="."/></author-name>
  </xsl:template>

  <!-- Named utility template (never called — for unused detection) -->
  <xsl:template name="f:format-date">
    <xsl:param name="date"/>
    <formatted><xsl:value-of select="$date"/></formatted>
  </xsl:template>

  <!-- Named function -->
  <xsl:function name="f:normalize-text">
    <xsl:param name="text"/>
    <xsl:value-of select="normalize-space($text)"/>
  </xsl:function>

</xsl:stylesheet>
