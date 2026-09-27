import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url))
const webDirectory = path.resolve(scriptDirectory, '..')
const repositoryDirectory = path.resolve(webDirectory, '..')
const windows = process.platform === 'win32'
const python = path.join(
  repositoryDirectory,
  'tools',
  'game-data-exporter',
  '.venv',
  windows ? 'Scripts/python.exe' : 'bin/python',
)

const forwarded = process.argv.slice(2)
const helpRequested = forwarded.includes('--help') || forwarded.includes('-h')
if (!existsSync(python)) {
  process.stderr.write(
    `Exporter Python environment not found at ${python}. See tools/game-data-exporter/README.md for setup.\n`,
  )
  process.exit(2)
}

const hasOption = (name) =>
  forwarded.some(
    (argument) => argument === name || argument.startsWith(`${name}=`),
  )
for (let index = 0; index < forwarded.length; index += 1) {
  const option = ['--asset-output', '--source', '--output'].find(
    (name) =>
      forwarded[index] === name || forwarded[index].startsWith(`${name}=`),
  )
  if (!option) continue

  if (forwarded[index] === option) {
    const value = forwarded[index + 1]
    if (value && !value.startsWith('--') && !path.isAbsolute(value)) {
      forwarded[index + 1] = path.resolve(webDirectory, value)
    }
    index += 1
  } else {
    const value = forwarded[index].slice(option.length + 1)
    if (value && !path.isAbsolute(value)) {
      forwarded[index] = `${option}=${path.resolve(webDirectory, value)}`
    }
  }
}
const exporterArguments = ['-m', 'phmon_game_exporter.cli', 'export']

if (!helpRequested) {
  if (!hasOption('--source') && process.env.GREATESTSRO_SOURCE) {
    const sourcePath = process.env.GREATESTSRO_SOURCE
    exporterArguments.push(
      '--source',
      path.isAbsolute(sourcePath)
        ? sourcePath
        : path.resolve(webDirectory, sourcePath),
    )
  }
  if (!hasOption('--output')) {
    exporterArguments.push(
      '--output',
      path.join(repositoryDirectory, 'exports', 'greatestsro'),
    )
  }
  if (!hasOption('--asset-output')) {
    const assetOutput = process.env.PHMON_GAME_ASSETS_OUTPUT
      ? path.resolve(webDirectory, process.env.PHMON_GAME_ASSETS_OUTPUT)
      : path.join(webDirectory, 'public', 'game-assets')
    exporterArguments.push('--asset-output', assetOutput)
  }
}
exporterArguments.push(...forwarded)

const result = spawnSync(python, exporterArguments, {
  cwd: repositoryDirectory,
  env: process.env,
  stdio: 'inherit',
  windowsHide: true,
})
if (result.error) {
  process.stderr.write(`${result.error.message}\n`)
  process.exit(1)
}
process.exit(result.status ?? 1)
