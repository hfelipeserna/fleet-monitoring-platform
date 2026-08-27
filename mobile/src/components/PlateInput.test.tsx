import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { PlateInput } from './PlateInput';

// Covers [SPEC-005: AC-001] - FR-001 BR-001/002 TS-001: Plate input normalize + validate + Connect state
describe('PlateInput', () => {
  describe('when typing valid plate acf356', () => {
    it('normalizes to ACF356 and enables Connect with green #86efac', () => {
      // Arrange
      const onConnect = jest.fn();
      const { getByTestId, getByDisplayValue } = render(<PlateInput onConnect={onConnect} />);
      const input = getByTestId('plate-input');
      const connectBtn = getByTestId('connect-btn');

      // Act
      fireEvent.changeText(input, 'acf356');

      // Assert
      expect(getByDisplayValue('ACF356')).toBeTruthy();
      (expect(connectBtn) as any).not.toBeDisabled();
      // Connect enabled verde #86efac per FR-012
      const style = connectBtn.props.style;
      const flat = Array.isArray(style) ? Object.assign({}, ...style) : style;
      // allow either backgroundColor prop or style object
      const bg = flat?.backgroundColor ?? connectBtn.props?.style?.backgroundColor;
      const hasGreen = JSON.stringify(style).includes('#86efac') || bg === '#86efac';
      expect(hasGreen).toBe(true);
      expect(() => getByDisplayValue('ACF356')).not.toThrow();
    });

    it('pressing Connect with valid plate calls onConnect with normalized plate', () => {
      // Arrange
      const onConnect = jest.fn();
      const { getByTestId } = render(<PlateInput onConnect={onConnect} />);
      const input = getByTestId('plate-input');
      const connectBtn = getByTestId('connect-btn');
      fireEvent.changeText(input, 'acf356');

      // Act
      fireEvent.press(connectBtn);

      // Assert
      expect(onConnect).toHaveBeenCalledTimes(1);
      expect(onConnect).toHaveBeenCalledWith('ACF356');
    });
  });

  describe('when typing invalid plate ACF35', () => {
    it('shows hint "3 letras + 3 números" and keeps Connect disabled', () => {
      // Arrange
      const onConnect = jest.fn();
      const { getByTestId, getByText } = render(<PlateInput onConnect={onConnect} />);
      const input = getByTestId('plate-input');
      const connectBtn = getByTestId('connect-btn');

      // Act
      fireEvent.changeText(input, 'ACF35');

      // Assert
      expect(getByText('3 letras + 3 números')).toBeTruthy();
      (expect(connectBtn) as any).toBeDisabled();
    });

    it('pressing Connect while invalid does not call onConnect', () => {
      // Arrange
      const onConnect = jest.fn();
      const { getByTestId } = render(<PlateInput onConnect={onConnect} />);
      const input = getByTestId('plate-input');
      const connectBtn = getByTestId('connect-btn');
      fireEvent.changeText(input, 'ACF35');

      // Act
      fireEvent.press(connectBtn);

      // Assert
      expect(onConnect).not.toHaveBeenCalled();
    });
  });

  describe('when no plate (idle)', () => {
    it('does not call onConnect and Connect is disabled', () => {
      // Arrange
      const onConnect = jest.fn();
      const { getByTestId, queryByText } = render(<PlateInput onConnect={onConnect} />);
      const connectBtn = getByTestId('connect-btn');

      // Act
      fireEvent.press(connectBtn);

      // Assert
      expect(onConnect).not.toHaveBeenCalled();
      (expect(connectBtn) as any).toBeDisabled();
      expect(queryByText('3 letras + 3 números')).toBeNull();
    });

    it('does not render hint when input empty', () => {
      // Arrange
      const { queryByText } = render(<PlateInput onConnect={jest.fn()} />);

      // Act
      // no typing

      // Assert
      expect(queryByText('3 letras + 3 números')).toBeNull();
    });
  });

  describe('input constraints', () => {
    it('has maxLength 6', () => {
      // Arrange
      const { getByTestId } = render(<PlateInput onConnect={jest.fn()} />);
      const input = getByTestId('plate-input');

      // Act
      const maxLength = input.props.maxLength;

      // Assert
      expect(maxLength).toBe(6);
    });

    it('has autoCapitalize characters', () => {
      // Arrange
      const { getByTestId } = render(<PlateInput onConnect={jest.fn()} />);
      const input = getByTestId('plate-input');

      // Act
      const autoCapitalize = input.props.autoCapitalize;

      // Assert
      expect(autoCapitalize).toBe('characters');
    });

    it('normalizes "  acf356 " with trim and uppercase on change', () => {
      // Arrange
      const { getByTestId, getByDisplayValue } = render(<PlateInput onConnect={jest.fn()} />);
      const input = getByTestId('plate-input');

      // Act
      fireEvent.changeText(input, '  acf356 ');

      // Assert
      expect(getByDisplayValue('ACF356')).toBeTruthy();
    });
  });
});
