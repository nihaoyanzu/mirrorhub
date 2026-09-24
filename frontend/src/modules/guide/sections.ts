import type { Component } from 'vue'
import GuideDocker from './GuideDocker.vue'
import GuideGoproxy from './GuideGoproxy.vue'
import GuideHuggingFace from './GuideHuggingFace.vue'
import GuideMaven from './GuideMaven.vue'
import GuideNpm from './GuideNpm.vue'
import GuidePypi from './GuidePypi.vue'

/** 公开说明页 section 组件表；与 MODULES id 对齐。 */
export const GUIDE_SECTIONS: Record<string, Component> = {
  pypi: GuidePypi,
  npm: GuideNpm,
  docker: GuideDocker,
  goproxy: GuideGoproxy,
  huggingface: GuideHuggingFace,
  maven: GuideMaven,
}
