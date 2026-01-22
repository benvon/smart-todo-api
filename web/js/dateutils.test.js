// Tests for dateutils.js
import { test } from 'node:test';
import assert from 'node:assert';
import { parseNaturalDate, formatDate, isValidDate, extractDateFromText, isDateOnly } from './dateutils.js';

test('parseNaturalDate - tomorrow', () => {
    const result = parseNaturalDate('tomorrow');
    assert(result !== null, 'Should parse "tomorrow"');
    const date = new Date(result);
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(0, 0, 0, 0);
    assert.strictEqual(date.getDate(), tomorrow.getDate(), 'Should be tomorrow');
    assert.strictEqual(date.getHours(), 0, 'Should be midnight (date-only)');
});

test('parseNaturalDate - tomorrow with time', () => {
    const result = parseNaturalDate('tomorrow at 3pm');
    assert(result !== null, 'Should parse "tomorrow at 3pm"');
    const date = new Date(result);
    assert.strictEqual(date.getHours(), 15, 'Should be 3pm (15:00)');
});

test('parseNaturalDate - next Friday', () => {
    const result = parseNaturalDate('next Friday');
    assert(result !== null, 'Should parse "next Friday"');
    const date = new Date(result);
    assert.strictEqual(date.getDay(), 5, 'Should be Friday (day 5)');
    assert.strictEqual(date.getHours(), 0, 'Should be midnight (date-only)');
});

test('parseNaturalDate - this weekend', () => {
    const result = parseNaturalDate('this weekend');
    assert(result !== null, 'Should parse "this weekend"');
});

test('parseNaturalDate - in 3 days', () => {
    const result = parseNaturalDate('in 3 days');
    assert(result !== null, 'Should parse "in 3 days"');
    const date = new Date(result);
    const expected = new Date();
    expected.setDate(expected.getDate() + 3);
    expected.setHours(0, 0, 0, 0);
    assert.strictEqual(date.getDate(), expected.getDate(), 'Should be 3 days from now');
});

test('parseNaturalDate - invalid input', () => {
    assert.strictEqual(parseNaturalDate(''), null, 'Empty string should return null');
    assert.strictEqual(parseNaturalDate(null), null, 'Null should return null');
    assert.strictEqual(parseNaturalDate('not a date'), null, 'Invalid input should return null');
});

test('formatDate - today', () => {
    const today = new Date();
    today.setHours(14, 30, 0, 0);
    const formatted = formatDate(today.toISOString());
    assert(formatted.includes('Today'), 'Should include "Today"');
    assert(formatted.includes('2:30 PM') || formatted.includes('14:30'), 'Should include time');
});

test('formatDate - tomorrow', () => {
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(10, 0, 0, 0);
    const formatted = formatDate(tomorrow.toISOString());
    assert(formatted.includes('Tomorrow'), 'Should include "Tomorrow"');
});

test('formatDate - invalid input', () => {
    assert.strictEqual(formatDate(''), '', 'Empty string should return empty string');
    assert.strictEqual(formatDate('invalid'), '', 'Invalid date should return empty string');
});

test('isValidDate', () => {
    assert.strictEqual(isValidDate('tomorrow'), true, 'Should validate "tomorrow"');
    assert.strictEqual(isValidDate('invalid'), false, 'Should reject invalid input');
    assert.strictEqual(isValidDate(''), false, 'Should reject empty string');
});

test('extractDateFromText - date in text', () => {
    const result = extractDateFromText('Buy groceries tomorrow');
    assert(result.detectedDate !== null, 'Should detect date');
    assert(result.cleanedText.includes('Buy groceries'), 'Should clean text');
    assert(!result.cleanedText.includes('tomorrow'), 'Should remove date from text');
});

test('extractDateFromText - no date', () => {
    const result = extractDateFromText('Buy groceries');
    assert.strictEqual(result.detectedDate, null, 'Should not detect date');
    assert.strictEqual(result.cleanedText, 'Buy groceries', 'Should return original text');
});

test('extractDateFromText - multiple dates', () => {
    // Should use first detected date
    const result = extractDateFromText('Meeting tomorrow and next Friday');
    assert(result.detectedDate !== null, 'Should detect at least one date');
});

test('isDateOnly - midnight date', () => {
    const date = new Date();
    date.setHours(0, 0, 0, 0);
    assert.strictEqual(isDateOnly(date.toISOString()), true, 'Midnight should be date-only');
});

test('isDateOnly - date with time', () => {
    const date = new Date();
    date.setHours(14, 30, 0, 0);
    assert.strictEqual(isDateOnly(date.toISOString()), false, 'Date with time should not be date-only');
});
