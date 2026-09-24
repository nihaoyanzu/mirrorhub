/** 前端模块 UI 描述符：加平台时优先改此表，少改 Platform/Guide 分支。 */

export type PrefetchUI = 'none' | 'hint' | 'arch' | 'pypiWheel'

export type ThirdField = 'none' | 'metadata' | 'auth' | 'sumdb' | 'token'

export interface ModuleDescriptor {
  id: string
  labelKey: string
  /** 公开说明页 Tab 标题（品牌短名，与语言无关） */
  guideTitle: string
  enableLabelKey: string
  showPrefetchTab: boolean
  /** 公开说明页 API 失败时的兜底 enabled */
  guideDefaultEnabled: boolean
  defaults: {
    upstream: string
    file_upstream: string
    metadata_upstream?: string
  }
  fields: {
    thirdField: ThirdField
    showAccessTest: boolean
    persistUpstreamToken: boolean
  }
  labels: {
    upstreamKey: string
    fileUpstreamKey: string
    thirdFieldKey?: string
  }
  hints: {
    upstreamHintKey?: string
    prefetchHintKey?: string
    thirdFieldHintKey?: string
    thirdFieldPlaceholder?: string
  }
  prefetchUI: PrefetchUI
}

export const MODULES: ModuleDescriptor[] = [
  {
    id: 'pypi',
    labelKey: 'platform.modulePyPI',
    guideTitle: 'PyPI',
    enableLabelKey: 'platform.enablePyPI',
    showPrefetchTab: true,
    guideDefaultEnabled: true,
    defaults: {
      upstream: 'https://mirrors.aliyun.com/pypi',
      file_upstream: 'https://mirrors.aliyun.com/pypi',
      metadata_upstream: '',
    },
    fields: {
      thirdField: 'metadata',
      showAccessTest: true,
      persistUpstreamToken: false,
    },
    labels: {
      upstreamKey: 'platform.indexUpstream',
      fileUpstreamKey: 'platform.fileUpstream',
      thirdFieldKey: 'platform.metadataUpstream',
    },
    hints: {
      thirdFieldHintKey: 'platform.metadataHint',
      thirdFieldPlaceholder: 'PEP 658；留空回退到包文件上游',
    },
    prefetchUI: 'pypiWheel',
  },
  {
    id: 'npm',
    labelKey: 'platform.moduleNpm',
    guideTitle: 'npm',
    enableLabelKey: 'platform.enableNpm',
    showPrefetchTab: false,
    guideDefaultEnabled: true,
    defaults: {
      upstream: 'https://registry.npmmirror.com',
      file_upstream: 'https://registry.npmmirror.com',
      metadata_upstream: '',
    },
    fields: {
      thirdField: 'none',
      showAccessTest: false,
      persistUpstreamToken: false,
    },
    labels: {
      upstreamKey: 'platform.indexUpstream',
      fileUpstreamKey: 'platform.fileUpstream',
    },
    hints: {
      upstreamHintKey: 'platform.npmUpstreamHint',
      prefetchHintKey: 'platform.npmPrefetchHint',
    },
    prefetchUI: 'none',
  },
  {
    id: 'docker',
    labelKey: 'platform.moduleDocker',
    guideTitle: 'Docker',
    enableLabelKey: 'platform.enableDocker',
    showPrefetchTab: true,
    guideDefaultEnabled: true,
    defaults: {
      upstream: 'https://registry-1.docker.io',
      file_upstream: 'https://registry-1.docker.io',
      metadata_upstream: 'https://auth.docker.io',
    },
    fields: {
      thirdField: 'auth',
      showAccessTest: false,
      persistUpstreamToken: false,
    },
    labels: {
      upstreamKey: 'platform.registryUpstream',
      fileUpstreamKey: 'platform.blobUpstream',
      thirdFieldKey: 'platform.authUpstream',
    },
    hints: {
      upstreamHintKey: 'platform.dockerUpstreamHint',
      prefetchHintKey: 'platform.dockerPrefetchHint',
      thirdFieldHintKey: 'platform.authUpstreamHint',
      thirdFieldPlaceholder: 'https://auth.docker.io',
    },
    prefetchUI: 'arch',
  },
  {
    id: 'goproxy',
    labelKey: 'platform.moduleGoproxy',
    guideTitle: 'Go',
    enableLabelKey: 'platform.enableGoproxy',
    showPrefetchTab: true,
    guideDefaultEnabled: true,
    defaults: {
      upstream: 'https://goproxy.cn',
      file_upstream: 'https://goproxy.cn',
      metadata_upstream: 'https://goproxy.cn',
    },
    fields: {
      thirdField: 'sumdb',
      showAccessTest: false,
      persistUpstreamToken: false,
    },
    labels: {
      upstreamKey: 'platform.moduleUpstream',
      fileUpstreamKey: 'platform.moduleFileUpstream',
      thirdFieldKey: 'platform.sumdbUpstream',
    },
    hints: {
      upstreamHintKey: 'platform.goproxyUpstreamHint',
      prefetchHintKey: 'platform.goproxyPrefetchHint',
      thirdFieldHintKey: 'platform.sumdbUpstreamHint',
      thirdFieldPlaceholder: 'https://goproxy.cn',
    },
    prefetchUI: 'hint',
  },
  {
    id: 'huggingface',
    labelKey: 'platform.moduleHuggingFace',
    guideTitle: 'Hugging Face',
    enableLabelKey: 'platform.enableHuggingFace',
    showPrefetchTab: true,
    guideDefaultEnabled: true,
    defaults: {
      upstream: 'https://huggingface.co',
      file_upstream: 'https://huggingface.co',
      metadata_upstream: '',
    },
    fields: {
      thirdField: 'token',
      showAccessTest: false,
      persistUpstreamToken: true,
    },
    labels: {
      upstreamKey: 'platform.hubUpstream',
      fileUpstreamKey: 'platform.hubFileUpstream',
      thirdFieldKey: 'platform.upstreamToken',
    },
    hints: {
      upstreamHintKey: 'platform.huggingfaceUpstreamHint',
      prefetchHintKey: 'platform.huggingfacePrefetchHint',
      thirdFieldHintKey: 'platform.upstreamTokenHint',
    },
    prefetchUI: 'hint',
  },
  {
    id: 'maven',
    labelKey: 'platform.moduleMaven',
    guideTitle: 'Maven',
    enableLabelKey: 'platform.enableMaven',
    showPrefetchTab: true,
    guideDefaultEnabled: false,
    defaults: {
      upstream: 'https://maven.aliyun.com/repository/central',
      file_upstream: 'https://maven.aliyun.com/repository/central',
      metadata_upstream: '',
    },
    fields: {
      thirdField: 'none',
      showAccessTest: false,
      persistUpstreamToken: false,
    },
    labels: {
      upstreamKey: 'platform.mavenRepoUpstream',
      fileUpstreamKey: 'platform.mavenFileUpstream',
    },
    hints: {
      upstreamHintKey: 'platform.mavenUpstreamHint',
      prefetchHintKey: 'platform.mavenPrefetchHint',
    },
    prefetchUI: 'hint',
  },
]

export const MODULE_BY_ID: Record<string, ModuleDescriptor> = Object.fromEntries(
  MODULES.map((d) => [d.id, d]),
)

export function isKnownModule(id: string): boolean {
  return id in MODULE_BY_ID
}

export function guideFallbackModules(): { id: string; enabled: boolean }[] {
  return MODULES.map((d) => ({ id: d.id, enabled: d.guideDefaultEnabled }))
}
