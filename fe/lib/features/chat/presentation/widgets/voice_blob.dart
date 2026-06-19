import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';

/// Il blob conversazionale sfumato viola/rosa/azzurro delle schermate di
/// riferimento. Pulsa lentamente quando [active] (ascolto/invio in corso).
class VoiceBlob extends StatefulWidget {
  const VoiceBlob({super.key, this.active = false, this.size = 180});

  final bool active;
  final double size;

  @override
  State<VoiceBlob> createState() => _VoiceBlobState();
}

class _VoiceBlobState extends State<VoiceBlob>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller = AnimationController(
    vsync: this,
    duration: const Duration(seconds: 4),
  )..repeat();

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
        final t = _controller.value * 2 * math.pi;
        // Lieve pulsazione: più marcata quando attivo.
        final pulse = widget.active ? 0.08 : 0.03;
        final scale = 1 + math.sin(t) * pulse;
        return Transform.scale(
          scale: scale,
          child: Container(
            width: widget.size,
            height: widget.size,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              gradient: SweepGradient(
                transform: GradientRotation(t),
                // Ripetiamo il primo colore in coda per chiudere il giro senza
                // stacco netto.
                colors: [...AppColors.blob, AppColors.blob[0]],
              ),
            ),
            // Sfocatura morbida tramite ImageFiltered sul gradiente.
            child: _BlurOverlay(size: widget.size),
          ),
        );
      },
    );
  }
}

/// Sfocatura applicata sopra il gradiente per l'effetto "nebbia" delle immagini.
class _BlurOverlay extends StatelessWidget {
  const _BlurOverlay({required this.size});
  final double size;

  @override
  Widget build(BuildContext context) {
    return ClipOval(
      child: Container(
        decoration: BoxDecoration(
          shape: BoxShape.circle,
          gradient: RadialGradient(
            colors: [
              Colors.white.withValues(alpha: 0.0),
              Colors.white.withValues(alpha: 0.35),
            ],
            stops: const [0.5, 1.0],
          ),
        ),
      ),
    );
  }
}
