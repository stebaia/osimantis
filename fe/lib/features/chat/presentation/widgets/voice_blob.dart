import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:wave_blob/wave_blob.dart';

import '../../../../core/theme/app_colors.dart';

/// Blob conversazionale animato e sfumato (viola/rosa/azzurro) costruito con
/// il pacchetto `wave_blob`.
///
/// `WaveBlob` non ha un proprio loop di animazione, quindi usiamo un
/// [AnimationController] che forza il repaint ad ogni frame.
class VoiceBlob extends StatefulWidget {
  const VoiceBlob({super.key, this.active = false, this.size = 180});

  final bool active;
  final double size;

  @override
  State<VoiceBlob> createState() => _VoiceBlobState();
}

class _VoiceBlobState extends State<VoiceBlob>
    with SingleTickerProviderStateMixin {
  // Il blob ondeggia più veloce quando è attivo (ascolto/elaborazione), più lento
  // a riposo. Cambiamo la durata del controller in base ad `active`.
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: _durationFor(widget.active),
  )..repeat();

  static Duration _durationFor(bool active) =>
      active ? const Duration(milliseconds: 900) : const Duration(seconds: 3);

  @override
  void didUpdateWidget(VoiceBlob oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.active != widget.active) {
      _controller
        ..duration = _durationFor(widget.active)
        ..repeat();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _controller,
      builder: (context, _) {
        // Center + box quadrato fisso: il blob resta centrato e non "scivola"
        // quando l'ampiezza cambia in stato attivo (autoScale di wave_blob può
        // disegnare in modo asimmetrico dentro il box).
        return Center(
          child: ImageFiltered(
          imageFilter: ImageFilter.blur(sigmaX: 10, sigmaY: 10),
          child: SizedBox(
            width: widget.size,
            height: widget.size,
            child: WaveBlob(
              blobCount: 3,
              amplitude: widget.active ? 6500.0 : 3500.0,
              speed: 6.0,
              scale: 1.05,
              autoScale: true,
              // Rimuoviamo il cerchio centrale statico: vogliamo solo il blob.
              centerCircle: false,
              overCircle: false,
              colors: AppColors.blob
                  .map((color) => color.withValues(alpha: 0.9))
                  .toList(),
              child: const SizedBox.shrink(),
            ),
          ),
          ),
        );
      },
    );
  }
}
