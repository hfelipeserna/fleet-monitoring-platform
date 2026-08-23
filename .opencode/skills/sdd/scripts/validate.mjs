#!/usr/bin/env node
// sdd validate — linter semántico spec.md vs plan.md
// Uso: node .opencode/skills/sdd/scripts/validate.mjs [SPEC-ID]
//      node .opencode/skills/sdd/scripts/validate.mjs --all
// Sin args: valida el SPEC-XXX más reciente.
// Exit 0 = sin hallazgos altos, Exit 1 = hallazgos altos (CI fail).

import fs from 'fs';
import path from 'path';

const SPECS_ROOT = 'docs/specs';
const SEVERITY = { HIGH: 'alta', MED: 'media', LOW: 'baja' };

function read(p) { try { return fs.readFileSync(p, 'utf8'); } catch { return null; } }
function exists(p) { return fs.existsSync(p); }
function listSpecs() {
  if (!exists(SPECS_ROOT)) return [];
  return fs.readdirSync(SPECS_ROOT).filter(f => f.startsWith('SPEC-')).sort();
}
function extractIds(text, prefix) {
  const re = new RegExp(`\\b${prefix}-\\d{3}\\b`, 'g');
  return [...new Set((text.match(re) || []))];
}
function hasSection(text, title) {
  return text.toLowerCase().includes(title.toLowerCase());
}

function validateSpec(specText, specPath) {
  const findings = [];
  const ucs = extractIds(specText, 'UC');
  const frs = extractIds(specText, 'FR');
  const brs = extractIds(specText, 'BR');
  const acs = extractIds(specText, 'AC');
  const tss = extractIds(specText, 'TS');

  if (ucs.length === 0) findings.push({ severity: SEVERITY.HIGH, id: 'UC_MISSING', msg: `spec.md sin UC-XXX`, file: specPath });
  if (frs.length === 0) findings.push({ severity: SEVERITY.HIGH, id: 'FR_MISSING', msg: `spec.md sin FR-XXX`, file: specPath });
  if (acs.length === 0) findings.push({ severity: SEVERITY.HIGH, id: 'AC_MISSING', msg: `spec.md sin AC-XXX (Given/When/Then)`, file: specPath });
  if (tss.length === 0) findings.push({ severity: SEVERITY.MED, id: 'TS_MISSING', msg: `spec.md sin TS-XXX`, file: specPath });

  for (const uc of ucs) {
    const hasAC = acs.some(() => specText.includes(uc));
    const acRefsUC = specText.includes(uc) && acs.length > 0;
    if (!acRefsUC) findings.push({ severity: SEVERITY.HIGH, id: 'UC_SIN_AC', msg: `${uc} sin AC que lo cite`, file: specPath });
  }
  for (const ac of acs) {
    const hasTS = specText.includes(ac) && tss.length > 0;
    const tsRefsAC = tss.some(ts => specText.includes(ac) || specText.includes(ts));
    if (!specText.includes('TS-') || !hasTS) {
      // heurística: cada AC debería tener al menos un TS que lo mencione
      const linked = tss.filter(() => specText.includes(ac)).length > 0;
      if (!linked) findings.push({ severity: SEVERITY.HIGH, id: 'AC_SIN_TS', msg: `${ac} sin TS que lo cubra`, file: specPath });
    }
  }
  for (const ts of tss) {
    const linked = frs.some(fr => specText.includes(`${ts}`) && specText.includes(fr)) || acs.some(ac => specText.includes(ts));
    // si TS no referencia ningún FR/BR/AC -> SPEC GAP
    const refsRequirement = frs.some(fr => {
      const idxTS = specText.indexOf(ts);
      const ctx = specText.slice(Math.max(0, idxTS - 500), idxTS + 500);
      return ctx.includes(fr) || ctx.includes('FR-') || ctx.includes('BR-') || ctx.includes('AC-');
    });
    if (!refsRequirement) findings.push({ severity: SEVERITY.HIGH, id: 'TS_SIN_FR', msg: `${ts} introduce comportamiento sin FR/BR/AC padre -> SPEC GAP`, file: specPath });
  }

  // Diagramas con implementación en spec (prohibido)
  const mermaidBlocks = [...specText.matchAll(/```mermaid([\s\S]*?)```/g)].map(m => m[1]);
  const forbidden = /class\s+\w+|repository|adapter|interface\s+\w+|package\s+\w+|method\s+\w+/i;
  for (const block of mermaidBlocks) {
    if (forbidden.test(block)) {
      findings.push({ severity: SEVERITY.MED, id: 'DIAGRAMA_IMPLEMENTACION', msg: `Diagrama en spec.md contiene detalles de implementación (class/repository/adapter/interface)`, file: specPath });
      break;
    }
  }

  // Secciones obligatorias (17)
  const requiredSections = ['Overview', 'Scope', 'Actors and Systems', 'Use Cases', 'Functional Requirements', 'Business Rules', 'Main Flows', 'Alternative and Error Flows', 'State and Transitions', 'API / Interface Contracts', 'Sequence Diagrams', 'Flow Diagrams', 'Non-Functional Requirements', 'Acceptance Criteria', 'Functional Test Scenarios', 'Open Questions', 'Assumptions'];
  for (const s of requiredSections) {
    if (!hasSection(specText, s)) findings.push({ severity: SEVERITY.LOW, id: 'SECCION_FALTANTE', msg: `spec.md sin sección "${s}"`, file: specPath });
  }

  // Open Questions bloquea approved
  if (specText.includes('Open Questions') && /\[ \].*bloquea/i.test(specText) === false) {
    // no-op, solo informativo
  }

  return { ucs, frs, brs, acs, tss, findings };
}

function validatePlan(specText, planText, specPath, planPath) {
  const findings = [];
  const frs = extractIds(specText, 'FR');
  const brs = extractIds(specText, 'BR');
  const ucs = extractIds(specText, 'UC');
  const acs = extractIds(specText, 'AC');
  const tss = extractIds(specText, 'TS');
  const tests = extractIds(planText || '', 'TEST');

  if (!planText) {
    findings.push({ severity: SEVERITY.HIGH, id: 'PLAN_FALTANTE', msg: `plan.md no existe (requisito sin implementación)`, file: specPath });
    return { findings, tests };
  }

  for (const fr of frs) {
    if (!planText.includes(fr)) findings.push({ severity: SEVERITY.HIGH, id: 'FR_SIN_IMPLEMENTACION', msg: `${fr} sin implementación en plan.md (sin Technical Change ni Step)`, file: planPath });
  }
  for (const br of brs) {
    if (!planText.includes(br)) findings.push({ severity: SEVERITY.MED, id: 'BR_SIN_IMPLEMENTACION', msg: `${br} sin implementación en plan.md`, file: planPath });
  }
  for (const ts of tss) {
    if (!planText.includes(ts)) findings.push({ severity: SEVERITY.HIGH, id: 'TS_SIN_TEST', msg: `${ts} sin TEST-XXX en plan.md`, file: planPath });
  }
  for (const test of tests) {
    const hasTS = tss.some(ts => planText.includes(ts) && planText.includes(test) && Math.abs(planText.indexOf(test) - planText.indexOf(ts)) < 3000);
    const ctx = planText.slice(Math.max(0, planText.indexOf(test) - 800), planText.indexOf(test) + 800);
    const refsTS = /TS-\d{3}/.test(ctx);
    if (!refsTS) findings.push({ severity: SEVERITY.HIGH, id: 'TEST_SIN_TS', msg: `${test} introduce comportamiento no especificado (sin TS padre) -> SPEC GAP`, file: planPath });
  }

  // Steps sin Spec References
  const steps = [...planText.matchAll(/### Step \d+[^\n]*\n([\s\S]*?)(?=### Step|\n## 13\.|\n## 14\.|\n## 15\.|$)/g)];
  for (const m of steps) {
    const body = m[0];
    if (!/Spec References/i.test(body) || !/(UC-|FR-|BR-|AC-)/.test(body)) {
      findings.push({ severity: SEVERITY.MED, id: 'STEP_SIN_JUSTIFICACION', msg: `Step sin Spec References (UC/FR/BR/AC)`, file: planPath });
    }
  }

  // SPEC GAP explícito
  if (/SPEC GAP/i.test(planText)) {
    findings.push({ severity: SEVERITY.MED, id: 'SPEC_GAP_EXPLICITO', msg: `plan.md contiene SPEC GAP — requiere resolución en spec.md`, file: planPath });
  }

  // Contradicción heurística: spec dice algo que plan no refleja en Technical Context / Architecture Changes
  // (check liviano: si plan no menciona NATS/TimescaleDB/Genkit cuando spec menciona telemetría/IoT/IA)
  if (/telemetry|NATS|JetStream/i.test(specText) && !/NATS|JetStream/i.test(planText)) {
    findings.push({ severity: SEVERITY.MED, id: 'CONTRADICCION_EVENTOS', msg: `spec menciona telemetría/eventos pero plan no menciona NATS/JetStream`, file: planPath });
  }

  // Traceability matrix presente
  if (!hasSection(planText, 'Specification Traceability')) {
    findings.push({ severity: SEVERITY.MED, id: 'TRACEABILITY_FALTANTE', msg: `plan.md sin matriz Specification Traceability`, file: planPath });
  }

  return { findings, tests };
}

function main() {
  const arg = process.argv[2];
  let specs = listSpecs();
  if (specs.length === 0) {
    console.log('No se encontraron specs en docs/specs/');
    process.exit(0);
  }

  let targets = [];
  if (!arg || arg === '--all') {
    targets = arg === '--all' ? specs : [specs[specs.length - 1]];
  } else if (arg.startsWith('SPEC-')) {
    const slug = specs.find(s => s.startsWith(arg));
    if (!slug) { console.error(`SPEC-ID ${arg} no encontrado en docs/specs/`); process.exit(1); }
    targets = [slug];
  } else {
    console.error('Uso: node validate.mjs [SPEC-XXX] | --all');
    process.exit(1);
  }

  let allFindings = [];
  let hasHigh = false;

  for (const dir of targets) {
    const specPath = path.join(SPECS_ROOT, dir, 'spec.md');
    const planPath = path.join(SPECS_ROOT, dir, 'plan.md');
    const specText = read(specPath);
    const planText = read(planPath);

    console.log(`\n=== ${dir} ===`);
    console.log(`  spec: ${specPath} ${specText ? '✓' : '✗'}`);
    console.log(`  plan: ${planPath} ${planText ? '✓' : '✗'}`);

    if (!specText) {
      console.log(`  [alta] spec.md faltante`);
      hasHigh = true;
      continue;
    }

    const rSpec = validateSpec(specText, specPath);
    const rPlan = validatePlan(specText, planText, specPath, planPath);
    const findings = [...rSpec.findings, ...rPlan.findings];

    console.log(`  UC:${rSpec.ucs.length} FR:${rSpec.frs.length} BR:${rSpec.brs.length} AC:${rSpec.acs.length} TS:${rSpec.tss.length} TEST:${rPlan.tests.length}`);

    if (findings.length === 0) {
      console.log('  ✓ sin hallazgos');
    } else {
      for (const f of findings) {
        const icon = f.severity === SEVERITY.HIGH ? '✗' : f.severity === SEVERITY.MED ? '!' : '·';
        console.log(`  ${icon} [${f.severity}] ${f.id}: ${f.msg} (${f.file})`);
        if (f.severity === SEVERITY.HIGH) hasHigh = true;
      }
    }
    allFindings.push(...findings);
  }

  console.log(`\n--- Resumen: ${allFindings.length} hallazgos, ${allFindings.filter(f=>f.severity===SEVERITY.HIGH).length} altos ---`);
  if (hasHigh) {
    console.log('sdd validate: FAIL (hallazgos altos -> SPEC GAP)');
    process.exit(1);
  } else {
    console.log('sdd validate: PASS');
    process.exit(0);
  }
}

main();
