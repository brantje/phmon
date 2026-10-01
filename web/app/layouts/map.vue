<template>
  <NuxtLayout name="default"><slot /></NuxtLayout>
</template>

<style>
.phmon-shell:has(.map-page) .workspace {
  display: flex;
  height: 100dvh;
  min-height: 0;
  flex-direction: column;
}
.phmon-shell:has(.map-page) .workspace-content {
  width: 100%;
  max-width: none;
  margin: 0;
  padding: 12px 16px 16px;
  flex: 1;
  min-height: 0;
}
@media (max-width: 899px) {
  .phmon-shell:has(.map-page) .workspace {
    height: auto;
    min-height: 100dvh;
  }
  .phmon-shell:has(.map-page) .workspace-content {
    padding: 8px;
  }
}
</style>
