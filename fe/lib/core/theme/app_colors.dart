import 'package:flutter/material.dart';

/// Palette derivata dalle schermate di riferimento: sfondo chiarissimo, accento
/// viola, card bianche, testo grigio scuro. Il blob usa il gradiente [blob].
class AppColors {
  const AppColors._();

  static const Color primary = Color(0xFF7C6FE8); // viola accento
  static const Color primaryDark = Color(0xFF5B4FCF);

  static const Color background = Color(0xFFF6F6F8); // grigio chiarissimo
  static const Color surface = Color(0xFFFFFFFF); // card

  static const Color textPrimary = Color(0xFF1E1E24);
  static const Color textSecondary = Color(0xFF8A8A93);

  static const Color border = Color(0x14000000); // ombre/bordi morbidi

  /// Colori del blob conversazionale (sfumatura viola/rosa/azzurro).
  static const List<Color> blob = [
    Color(0xFFB8A6F0),
    Color(0xFFE9A6D8),
    Color(0xFFA6C8F0),
  ];
}
